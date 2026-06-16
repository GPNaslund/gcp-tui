# gcp-tui

A safe, transparent launcher for the [Cloud SQL Auth Proxy](https://cloud.google.com/sql/docs/postgres/sql-proxy) across multiple GCP environments.

It exists to kill two recurring problems when you query staging/prod databases from a laptop:

1. **"Which environment am I on?"** — every environment binds to a *distinct reserved loopback slot* (`127.0.0.2:15433`, `127.0.0.3:15434`, …), so a connection string structurally encodes its environment. Prod can never be mistaken for localhost.
2. **Auth friction** — it checks the two credential systems gcloud uses (your user login *and* Application Default Credentials, which the proxy actually needs) and offers to run the right `gcloud auth` flow when one is missing.

Every external command it runs is printed before it runs. Nothing it does to your gcloud or Cloud SQL state is hidden.

## Install

```sh
go install github.com/gpnaslund/gcp-tui@latest
```

Requires the [`gcloud` CLI](https://cloud.google.com/sdk/docs/install) and [`cloud-sql-proxy`](https://cloud.google.com/sql/docs/postgres/sql-proxy#install) on your `PATH`.

## Usage

```sh
gcp-tui doctor          # check prerequisites + auth, fix what's missing
gcp-tui init            # discover Cloud SQL instances via gcloud, add them
gcp-tui list            # show configured environments
gcp-tui up staging      # start the proxy for an environment
```

`init` queries gcloud for your projects and their Cloud SQL instances, auto-assigns the next free reserved slot, and defaults prod-looking environments to requiring typed confirmation. It only ever proposes entries — the config file is the source of truth.

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
confirm  = true     # require typing "prod" before the tunnel starts
```

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

## Safety model

- **Distinct loopback IP + port per environment.** The strongest layer — the address itself tells you the environment. On macOS, non-`127.0.0.1` loopback IPs need `sudo ifconfig lo0 alias 127.0.0.2 up` (Linux routes all of `127.0.0.0/8` by default).
- **Typed confirmation** for any environment with `confirm = true`.
- **Pre-flight slot check** — refuses to start if the reserved address:port already has a listener (stale proxy or a squatter).
- **Single source of truth** — declared config, never filesystem guessing.

## Status

v1: `doctor`, `init`, `list`, `up`. A Bubble Tea TUI front-end is planned on top of this core.
