// Package mcpserver exposes gcp-tui's environment-scoped operations as Model
// Context Protocol tools over stdio, so an agent can drive the same read surface
// the CLI offers. It calls the internal packages directly rather than shelling
// out to the binary, and routes every write through the shared write gate
// (internal/gate). Because an MCP server has no interactive terminal, that gate
// structurally refuses every write to a protected (confirm=true / prod)
// environment: an agent can read freely but can never mutate prod.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/gate"
	"github.com/gpnaslund/gcp-tui/internal/run"
)

// version is reported to clients in the server implementation info.
const version = "0.1.0"

const instructions = `gcp-tui exposes read-only inspection of configured Cloud SQL environments and a
small set of gated writes.

Reads are always allowed. Call list_environments first to discover environment
names and which are protected (confirm=true).

Writes (backups_create, secrets_set) are gated: a protected environment can
never be written through this server, because the gate requires an interactive
terminal that an MCP server does not have. A non-prod write also requires
authorize=true on the call. Secret values you pass to secrets_set travel through
this conversation; do not use it for values that must stay hidden from the agent.`

// Run builds the MCP server and serves it on stdio until the client disconnects
// or ctx is cancelled. Stdout is reserved for the JSON-RPC transport: tool
// handlers capture subprocess output and never print to it. Dry-run is forced
// off because it would print command lines to stdout and corrupt the transport.
func Run(ctx context.Context) error {
	run.DryRun = false
	return newServer().Run(ctx, &mcp.StdioTransport{})
}

// newServer constructs the server with all tools registered, without running it.
// Split out from Run so tests can drive it over an in-memory transport.
func newServer() *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: "gcp-tui", Version: version},
		&mcp.ServerOptions{Instructions: instructions},
	)
	registerTools(s)
	return s
}

// findEnv loads the config and resolves one environment by name.
func findEnv(name string) (*config.Env, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	env, ok := cfg.Find(name)
	if !ok {
		return nil, fmt.Errorf("no environment %q; call list_environments to see configured ones", name)
	}
	return env, nil
}

// authorize applies the write gate in the non-interactive MCP context. An MCP
// server has no terminal, so interactive is always false: a protected (prod)
// environment is therefore always refused, and a non-prod write proceeds only
// when the caller passed authorize=true.
func authorize(env config.Env, agentAuthorized bool) error {
	switch gate.Decide(env.Confirm, false /*interactive*/, agentAuthorized) {
	case gate.Allow:
		return nil
	default: // Refuse — TypedConfirm is unreachable without a terminal
		if env.Confirm {
			return fmt.Errorf("environment %q is protected (confirm=true); writes to it are refused over MCP — a prod write requires a human typing the environment name at a real terminal", env.Name)
		}
		return fmt.Errorf("write to %q requires explicit authorization; call again with authorize=true", env.Name)
	}
}

// resolveProfile selects the requested profile, or the only one when the
// environment has exactly one and none was named.
func resolveProfile(env *config.Env, name string) (*config.Profile, error) {
	if name != "" {
		p, ok := env.FindProfile(name)
		if !ok {
			return nil, fmt.Errorf("no profile %q on %q", name, env.Name)
		}
		return p, nil
	}
	switch len(env.Profiles) {
	case 0:
		return nil, fmt.Errorf("no profiles on %q", env.Name)
	case 1:
		return &env.Profiles[0], nil
	default:
		return nil, fmt.Errorf("%q has multiple profiles; name one", env.Name)
	}
}
