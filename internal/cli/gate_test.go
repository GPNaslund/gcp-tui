package cli

import (
	"testing"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

// stubGate swaps the gate's injectable seams for the duration of one test and
// restores them via t.Cleanup. confirmMatch is the value confirmFn returns when
// it is reached (only relevant on a prod+TTY row).
func stubGate(t *testing.T, tty, yes, confirmMatch bool) {
	t.Helper()
	origTTY, origConfirm, origYes := stdinIsTTY, confirmFn, flagYes
	t.Cleanup(func() {
		stdinIsTTY, confirmFn, flagYes = origTTY, origConfirm, origYes
	})
	stdinIsTTY = func() bool { return tty }
	confirmFn = func(string) bool { return confirmMatch }
	flagYes = yes
}

func TestAuthorizeWriteMatrix(t *testing.T) {
	prod := config.Env{Name: "prod", Confirm: true}
	staging := config.Env{Name: "staging", Confirm: false}

	cases := []struct {
		name         string
		env          config.Env
		tty          bool
		yes          bool
		confirmMatch bool
		wantErr      bool
	}{
		// THE INVARIANT: prod off a TTY is refused even with --yes.
		{"prod no-TTY no-yes refused", prod, false, false, false, true},
		{"prod no-TTY WITH --yes still refused", prod, false, true, true, true},

		// prod on a TTY defers to confirmFn.
		{"prod TTY confirm matches", prod, true, false, true, false},
		{"prod TTY confirm mismatches", prod, true, false, false, true},
		{"prod TTY --yes does not bypass mismatch", prod, true, true, false, true},

		// non-prod: TTY always ok; no-TTY needs --yes.
		{"non-prod TTY ok", staging, true, false, false, false},
		{"non-prod no-TTY no-yes refused", staging, false, false, false, true},
		{"non-prod no-TTY WITH --yes ok", staging, false, true, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stubGate(t, c.tty, c.yes, c.confirmMatch)
			err := authorizeWrite(c.env)
			if (err != nil) != c.wantErr {
				t.Fatalf("authorizeWrite(%+v) tty=%v yes=%v confirm=%v: err=%v, wantErr=%v",
					c.env, c.tty, c.yes, c.confirmMatch, err, c.wantErr)
			}
		})
	}
}

// TestAuthorizeWriteProdIgnoresYes is the safety-critical regression: --yes must
// never be sufficient to authorize a prod (Confirm=true) write off a terminal.
func TestAuthorizeWriteProdIgnoresYes(t *testing.T) {
	stubGate(t, false /*tty*/, true /*yes*/, true /*confirmMatch*/)
	if err := authorizeWrite(config.Env{Name: "prod", Confirm: true}); err == nil {
		t.Fatal("--yes authorized a prod write off a TTY; the prod invariant is broken")
	}
}
