package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

// writeConfigFixture points config.Load at a temp XDG dir holding one protected
// (prod, confirm=true) and one non-prod (staging) environment, so handlers that
// call config.Load resolve real environments without reading the developer's own
// config or touching any cloud.
func writeConfigFixture(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Envs: []config.Env{
		{
			Name: "staging", Project: "p", Instance: "p:r:staging",
			Address: "127.0.0.2", Port: 15433, Confirm: false,
			Profiles: []config.Profile{{Name: "app", User: "app", DBName: "db", SSLMode: "disable"}},
		},
		{
			Name: "prod", Project: "q", Instance: "q:r:prod",
			Address: "127.0.0.3", Port: 15434, Confirm: true,
			Profiles: []config.Profile{{Name: "app", User: "app", DBName: "db", SSLMode: "disable"}},
		},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save fixture config: %v", err)
	}
}

// connectTestClient starts the real server over an in-memory transport and
// returns a connected client session, cleaned up when the test ends.
func connectTestClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clientT, serverT := mcp.NewInMemoryTransports()
	go func() { _ = newServer().Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// resultText joins the text content of a tool result. For a tool error the
// handler's message is packed here; for a structured-output read it is the JSON
// rendering of the output.
func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// TestWriteGateRefusalsOverCallTool is the safety-critical end-to-end check: the
// write-gate refusal must survive a real tools/call round trip — argument
// decode, handler dispatch, and result packing — and surface as an in-band tool
// error, not slip through as a successful write. It complements the unit-level
// TestAuthorizeMCPMatrix by proving the refusal travels through the protocol.
// Every refusal returns before any gcloud exec, so this needs no cloud access.
func TestWriteGateRefusalsOverCallTool(t *testing.T) {
	writeConfigFixture(t)
	session := connectTestClient(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		tool     string
		args     map[string]any
		wantText string
	}{
		{"backups_create prod refused", "backups_create",
			map[string]any{"env": "prod"}, "protected"},
		// THE INVARIANT: authorize=true must not unlock a protected environment.
		{"backups_create prod refused even with authorize", "backups_create",
			map[string]any{"env": "prod", "authorize": true}, "protected"},
		{"secrets_set prod refused even with authorize", "secrets_set",
			map[string]any{"env": "prod", "name": "x", "value": "y", "authorize": true}, "protected"},
		{"backups_create non-prod needs authorize", "backups_create",
			map[string]any{"env": "staging"}, "authorize=true"},
		{"secrets_set non-prod needs authorize", "secrets_set",
			map[string]any{"env": "staging", "name": "x", "value": "y"}, "authorize=true"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: c.tool, Arguments: c.args})
			if err != nil {
				t.Fatalf("got a protocol error, want an in-band tool error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("%s: expected a refusal (IsError), got success: %s", c.tool, resultText(res))
			}
			if got := resultText(res); !strings.Contains(got, c.wantText) {
				t.Fatalf("%s: refusal %q does not mention %q", c.tool, got, c.wantText)
			}
		})
	}
}

// TestListEnvironmentsOverCallTool proves the read path is wired end-to-end: a
// tools/call reaches the handler, the empty input decodes, and the typed output
// is returned and rendered as content. list_environments reads only the config,
// so it needs no cloud access.
func TestListEnvironmentsOverCallTool(t *testing.T) {
	writeConfigFixture(t)
	session := connectTestClient(t)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_environments"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("list_environments errored: %s", resultText(res))
	}
	text := resultText(res)
	for _, want := range []string{"staging", "prod"} {
		if !strings.Contains(text, want) {
			t.Errorf("result missing env %q; got: %s", want, text)
		}
	}
}
