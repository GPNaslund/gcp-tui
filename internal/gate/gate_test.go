package gate

import "testing"

func TestDecideMatrix(t *testing.T) {
	cases := []struct {
		name        string
		confirm     bool
		interactive bool
		assumeYes   bool
		want        Decision
	}{
		// THE INVARIANT: prod off a terminal is refused, even with assumeYes.
		{"prod no-tty no-yes refused", true, false, false, Refuse},
		{"prod no-tty WITH yes still refused", true, false, true, Refuse},

		// prod on a terminal defers to a typed confirmation; yes never bypasses it.
		{"prod tty needs typed confirm", true, true, false, TypedConfirm},
		{"prod tty yes still needs typed confirm", true, true, true, TypedConfirm},

		// non-prod: a terminal always allows; off-terminal needs the escape hatch.
		{"non-prod tty ok", false, true, false, Allow},
		{"non-prod no-tty no-yes refused", false, false, false, Refuse},
		{"non-prod no-tty WITH yes ok", false, false, true, Allow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Decide(c.confirm, c.interactive, c.assumeYes); got != c.want {
				t.Fatalf("Decide(confirm=%v, interactive=%v, assumeYes=%v) = %v, want %v",
					c.confirm, c.interactive, c.assumeYes, got, c.want)
			}
		})
	}
}
