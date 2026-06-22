package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

// TestAuthorizeMCPMatrix is the safety-critical check for the MCP write gate:
// over MCP there is no terminal, so a protected (prod) environment must be
// refused regardless of authorize, and a non-prod write needs authorize=true.
func TestAuthorizeMCPMatrix(t *testing.T) {
	prod := config.Env{Name: "prod", Confirm: true}
	staging := config.Env{Name: "staging", Confirm: false}

	cases := []struct {
		name    string
		env     config.Env
		auth    bool
		wantErr bool
	}{
		{"prod without authorize refused", prod, false, true},
		{"prod WITH authorize STILL refused", prod, true, true},
		{"non-prod without authorize refused", staging, false, true},
		{"non-prod WITH authorize ok", staging, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := authorize(c.env, c.auth)
			if (err != nil) != c.wantErr {
				t.Fatalf("authorize(%+v, auth=%v) err=%v, wantErr=%v", c.env, c.auth, err, c.wantErr)
			}
		})
	}
}

// TestServerExposesTools drives the real server over an in-memory transport: it
// proves the schema inference for every tool succeeds, the server initialises,
// and the expected tool set is advertised with the read tools hinted read-only.
func TestServerExposesTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientT, serverT := mcp.NewInMemoryTransports()
	go func() { _ = newServer().Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	readOnly := map[string]bool{}
	for _, tool := range res.Tools {
		readOnly[tool.Name] = tool.Annotations != nil && tool.Annotations.ReadOnlyHint
	}

	wantRead := []string{
		"status", "list_environments", "sql_databases", "sql_users",
		"sql_describe", "logs", "backups_list", "secrets_list", "connection_string",
	}
	for _, name := range wantRead {
		got, ok := readOnly[name]
		if !ok {
			t.Errorf("missing tool %q", name)
			continue
		}
		if !got {
			t.Errorf("tool %q should be hinted read-only", name)
		}
	}

	for _, name := range []string{"backups_create", "secrets_set"} {
		if got, ok := readOnly[name]; !ok {
			t.Errorf("missing write tool %q", name)
		} else if got {
			t.Errorf("write tool %q must not be hinted read-only", name)
		}
	}
}
