# gcp-tui

A safe, transparent launcher for the [Cloud SQL Auth Proxy](https://cloud.google.com/sql/docs/postgres/sql-proxy) across multiple GCP environments — with an interactive cockpit and an agent-friendly CLI.

It exists to kill two recurring problems when you query staging/prod databases from a laptop:

1. **"Which environment am I on?"** — every environment binds to a *distinct reserved loopback slot* (`127.0.0.2:15433`, `127.0.0.3:15434`, …), so a connection string structurally encodes its environment. Prod can never be mistaken for localhost.
2. **Auth friction** — it checks the two credential systems gcloud uses (your user login *and* Application Default Credentials, which the proxy actually needs), validates that neither has expired, and offers to run the right `gcloud auth` flow when one is missing.

Every external command it runs is printed before it runs. Nothing it does to your gcloud or Cloud SQL state is hidden.

## Install

```sh
go install github.com/gpnaslund/gcp-tui@latest
```

Requires the [`gcloud` CLI](https://cloud.google.com/sdk/docs/install) and [`cloud-sql-proxy`](https://cloud.google.com/sql/docs/postgres/sql-proxy#install) on your `PATH`.

## Usage

```sh
gcp-tui                       # launch the interactive TUI cockpit
gcp-tui doctor                # check prerequisites + auth, fix what's missing
gcp-tui status                # one-shot snapshot of auth, tools, and environments
gcp-tui init                  # discover Cloud SQL instances via gcloud, add them
gcp-tui list                  # show configured environments
gcp-tui up staging            # start the proxy for an environment (foreground)
gcp-tui down staging          # kill a stale listener on the environment's slot
gcp-tui logs staging          # tail Cloud Logging for the environment's instance
gcp-tui sql databases staging # list databases / users / describe the instance
gcp-tui backups list staging  # list (and, gated, create) on-demand backups
gcp-tui mcp                   # run the MCP server (stdio) for an agent to drive
```

## The cockpit

Run with no arguments to open the **TUI** — a two-pane cockpit (environments ↔ connection profiles) over the same core the CLI uses. The left pane lists environments with a live/idle slot pill; the right pane inspects the selected one (slot, instance, auth mode, profiles).

It's built to keep you working without leaving the screen:

- **Background tunnels.** `⏎` on an environment starts a *tracked background proxy* and auto-opens a **streaming log panel** seeded with the environment's connection string(s). The cockpit stays interactive, so several tunnels can be live at once. `x` stops the selected (or panelled) tunnel. Tunnels are **session-scoped**: every one this cockpit started is killed on quit, so a normal exit never leaks a proxy.
- **In-cockpit read panels** (async, non-blocking) on the selected environment: `L` logs · `D` databases · `U` users · `I` instance describe · `B` backups.
- **Prod gate.** Starting a tunnel for a `confirm = true` environment requires typing the environment name first — the cockpit equivalent of the CLI write gate. No tunnel runs for a protected env until the typed name matches.
- **Quick actions:** `c` copy a profile's connection string · `p` add a profile · `i` discover instances · `s` pull secrets · `d` re-run doctor. `tab`/`→` focuses the profile list (`⏎`/`c` copies there); `q` or `ctrl+c` quits.

The footer always shows the keys available in the current context.

## Commands

Beyond the proxy-control basics (`up`/`down`/`conn`/`profile`/`secrets`), the CLI exposes read surfaces over each environment's instance:

- **`status`** — a one-shot snapshot: the active gcloud account, whether ADC is present and valid, which tools are installed (`gcloud`/`cloud-sql-proxy`/`psql`), and the environment→slot map.
- **`logs <env>`** — `gcloud logging read` scoped to the environment's Cloud SQL instance, bounded by default. Flags: `--since` (gcloud freshness, default `1h`), `--severity` (e.g. `ERROR`), `--grep`, `--limit` (default `50`).
- **`sql databases|users|describe <env>`** — list databases, list users, or describe the instance (version, region, state, tier, availability, disk, backup config, IPs).
- **`backups list <env>`** — list automated and on-demand backups. **`backups create <env>`** takes an on-demand backup (a gated write — see below). There is deliberately **no restore**.

## Agent-friendly

The CLI is built so a non-interactive caller (a script, CI, or an LLM agent) can read freely and is *structurally* prevented from mutating production:

- **`--json`** — every read command (`status`, `logs`, `sql …`, `backups list`) emits machine-readable JSON instead of the human table.
- **`--dry-run`** — prints the exact `gcloud` command(s) that *would* run and exits without executing anything. Use it to see precisely what a command will do.
- **Non-interactive write gate** — reads are always allowed. Non-prod writes (e.g. `backups create`, `secrets set`) need `--yes` when there's no interactive terminal. For `confirm = true` (prod) environments the write is **refused outright without an interactive TTY — and `--yes` never authorizes prod**. The only path to a prod write is a human typing the environment name at a real terminal, so an agent cannot mutate prod no matter what flags it passes.

These flags are also the foundation the MCP server is built on (below).

## MCP server

`gcp-tui mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, exposing the same environment-scoped operations as typed tools so an agent can drive them directly — no shelling out, no parsing tables.

It is **headless by construction, and that is the safety mechanism**: an MCP server speaks JSON-RPC over stdio, so it has no interactive terminal. The write gate therefore refuses *every* write to a `confirm = true` (prod) environment — the exact invariant the CLI enforces, now holding for free. An agent can read any environment but can never mutate prod.

- **Reads** (always allowed): `status`, `list_environments`, `sql_databases`, `sql_users`, `sql_describe`, `logs`, `backups_list`, `secrets_list`, `connection_string`.
- **Gated writes**: `backups_create`, `secrets_set`. A protected environment is refused outright; a non-prod write additionally requires `authorize: true` on the call (the MCP equivalent of `--yes`).
- **No secret exfiltration by default**: `secrets_list` returns names only (no values), and `connection_string` omits the stored password unless you pass `include_password: true`.

Point an MCP client at it (for example, in a Claude Code / Claude Desktop config):

```json
{
  "mcpServers": {
    "gcp-tui": { "command": "gcp-tui", "args": ["mcp"] }
  }
}
```

## Config

Declared in `~/.config/gcp-tui/config.toml` (XDG-aware):

```toml
[[env]]
name     = "staging"
project  = "my-staging-project"
instance = "my-staging-project:europe-north2:my-staging-db"
address  = "127.0.0.2"
port     = 15433
iam_auth = false
confirm  = false

[[env]]
name     = "prod"
project  = "my-prod-project"
instance = "my-prod-project:europe-north2:my-prod-db"
address  = "127.0.0.3"
port     = 15434
iam_auth = false
confirm  = true     # require typing "prod" before a tunnel starts or a write runs
```

`init` queries gcloud for your projects and their Cloud SQL instances, auto-assigns the next free reserved slot, and defaults prod-looking environments to `confirm = true`. It only ever proposes entries — the config file is the source of truth.

Then connect with any client:

```sh
psql "postgres://USER@127.0.0.2:15433/DB"   # staging
psql "postgres://USER@127.0.0.3:15434/DB"   # prod
```

## Connection profiles

So you don't have to look up the user/database every time, attach **connection profiles** to an environment and get a ready-to-paste URL on demand:

```sh
gcp-tui profile add staging      # prompts for user, database, password
gcp-tui profile list             # show profiles (never the password)
gcp-tui conn staging             # print the connection string
gcp-tui conn staging app --copy  # copy a named profile to the clipboard
```

The **password is stored in your OS keyring** (Secret Service / Keychain / Credential Manager), never in the config file — only the user, database, and sslmode are declared in TOML. For IAM-auth environments no password is stored; the proxy injects the token.

The generated URL uses `sslmode=disable` because the local hop to the proxy is plaintext loopback (the proxy itself provides TLS to Cloud SQL); `sslmode=require` would fail against the proxy.

## Secret Manager

For environments backed by a GCP project you can manage Secret Manager values without leaving the tool:

```sh
gcp-tui secrets pull staging          # export selected secrets to a .env file
gcp-tui secrets set prod DB_PASSWORD  # write a new secret version (creates if missing)
gcp-tui secrets diff staging prod     # compare which secret names exist in each env
```

`secrets pull` offers a multi-select of the project's secrets and warns if the target `.env` isn't gitignored. `secrets set` prompts for the value with a masked input and pipes it to gcloud over stdin, so the secret never appears in your shell history or the process list; it is a gated write, honouring the same typed-confirmation rule as `up` for `confirm = true` environments.

## Safety model

- **Distinct loopback IP + port per environment.** The strongest layer — the address itself tells you the environment. On macOS, non-`127.0.0.1` loopback IPs need `sudo ifconfig lo0 alias 127.0.0.2 up` (Linux routes all of `127.0.0.0/8` by default).
- **Typed confirmation** for any environment with `confirm = true`, before a tunnel starts or a write runs.
- **Non-interactive write gate** — reads are unrestricted; non-prod writes need `--yes` off a TTY; prod writes are impossible without an interactive terminal, so automation can't reach production state.
- **Pre-flight slot check** — refuses to start if the reserved address:port already has a listener (stale proxy or a squatter).
- **Single source of truth** — declared config, never filesystem guessing.

## License

[MIT](LICENSE)
