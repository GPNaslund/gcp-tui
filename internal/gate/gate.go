// Package gate holds the write-authorization invariant in one place so every
// surface — the CLI and the MCP server — enforces it identically: a
// confirm=true ("prod") environment can be mutated only by an interactive typed
// confirmation at a real terminal. A non-interactive caller can never write to
// prod, and the --yes / authorize escape hatch applies only to non-prod
// environments.
package gate

// Decision is the outcome of applying the write gate to an environment.
type Decision int

const (
	// Refuse means the write must not proceed. It is the zero value so an
	// uninitialised Decision fails closed.
	Refuse Decision = iota
	// Allow means the write may proceed with no further confirmation.
	Allow
	// TypedConfirm means the caller must obtain an interactive typed match of
	// the environment name before proceeding (prod on a real terminal).
	TypedConfirm
)

// Decide applies the invariant. interactive reports whether a real interactive
// terminal is attached; assumeYes is the non-prod escape hatch (the CLI's --yes
// flag or the MCP authorize argument). assumeYes is deliberately never consulted
// for a confirm=true environment — that is what stops any non-interactive caller
// from mutating prod.
func Decide(confirm, interactive, assumeYes bool) Decision {
	if confirm {
		if interactive {
			return TypedConfirm
		}
		return Refuse // prod off a terminal is refused; assumeYes does not apply
	}
	if interactive || assumeYes {
		return Allow
	}
	return Refuse
}
