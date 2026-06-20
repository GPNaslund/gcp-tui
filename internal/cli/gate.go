package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

// stdinIsTTY reports whether stdin is an interactive terminal. It is a package
// var so gate_test.go can stub it without a real terminal.
var stdinIsTTY = func() bool { return isatty.IsTerminal(os.Stdin.Fd()) }

// confirmFn is the interactive prod prompt. It is a package var so gate_test.go
// can stub it without reading real stdin.
var confirmFn = typedConfirm

// authorizeWrite decides whether a state-changing action on env may proceed.
//
// THE INVARIANT: --yes NEVER authorizes a confirm=true (prod) environment. A
// prod write is permitted only by an interactive typed confirmation on a TTY;
// off a TTY it is refused outright, even with --yes. This is what stops an agent
// (or any non-interactive caller) from mutating prod.
//
// Matrix:
//
//	prod (Confirm)  + TTY     -> confirmFn(name): nil if it matches, else error
//	prod (Confirm)  + no TTY  -> error (refused even with --yes)
//	non-prod        + TTY     -> nil
//	non-prod        + no TTY  -> nil iff --yes, else error
//
// Reads never call this.
func authorizeWrite(env config.Env) error {
	if env.Confirm {
		if !stdinIsTTY() {
			return fmt.Errorf("refusing to write to protected environment %q without an interactive terminal; --yes does not apply to prod", env.Name)
		}
		if !confirmFn(env.Name) {
			return fmt.Errorf("aborted: confirmation did not match %q", env.Name)
		}
		return nil
	}

	if stdinIsTTY() {
		return nil
	}
	if flagYes {
		return nil
	}
	return fmt.Errorf("refusing to write to %q non-interactively; pass --yes to authorize", env.Name)
}
