package cli

import (
	"testing"
)

func TestFormatEnv(t *testing.T) {
	t.Run("sorted order and newline escape", func(t *testing.T) {
		got := formatEnv(map[string]string{
			"B": "x y",
			"A": "l1\nl2",
		})
		want := "A=\"l1\\nl2\"\nB=\"x y\"\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("backslash escape", func(t *testing.T) {
		got := formatEnv(map[string]string{
			"KEY": `back\slash`,
		})
		want := "KEY=\"back\\\\slash\"\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("double quote escape", func(t *testing.T) {
		got := formatEnv(map[string]string{
			"KEY": `say "hello"`,
		})
		want := "KEY=\"say \\\"hello\\\"\"\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("spaces and hash preserved inside quotes", func(t *testing.T) {
		got := formatEnv(map[string]string{
			"KEY": "hello world # not a comment",
		})
		want := "KEY=\"hello world # not a comment\"\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		got := formatEnv(map[string]string{})
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("combined escapes", func(t *testing.T) {
		// value with backslash + double-quote + newline, key order A before Z
		got := formatEnv(map[string]string{
			"Z": "plain",
			"A": "a\\b\"c\nd",
		})
		// A first (sorted), value: backslash->\\, quote->\", newline->\n
		want := "A=\"a\\\\b\\\"c\\nd\"\nZ=\"plain\"\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
