package cli

import (
	"reflect"
	"testing"
)

func TestDiffNames(t *testing.T) {
	cases := []struct {
		name      string
		a         []string
		b         []string
		wantOnlyA []string
		wantOnlyB []string
		wantBoth  []string
	}{
		{
			name:      "typical overlap",
			a:         []string{"C", "A", "B"},
			b:         []string{"B", "C", "D"},
			wantOnlyA: []string{"A"},
			wantOnlyB: []string{"D"},
			wantBoth:  []string{"B", "C"},
		},
		{
			name:      "both empty",
			a:         []string{},
			b:         []string{},
			wantOnlyA: nil,
			wantOnlyB: nil,
			wantBoth:  nil,
		},
		{
			name:      "full overlap",
			a:         []string{"X", "Y", "Z"},
			b:         []string{"Z", "X", "Y"},
			wantOnlyA: nil,
			wantOnlyB: nil,
			wantBoth:  []string{"X", "Y", "Z"},
		},
		{
			name:      "no overlap",
			a:         []string{"A", "B"},
			b:         []string{"C", "D"},
			wantOnlyA: []string{"A", "B"},
			wantOnlyB: []string{"C", "D"},
			wantBoth:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotOnlyA, gotOnlyB, gotBoth := diffNames(tc.a, tc.b)
			if !reflect.DeepEqual(gotOnlyA, tc.wantOnlyA) {
				t.Errorf("onlyA: got %v, want %v", gotOnlyA, tc.wantOnlyA)
			}
			if !reflect.DeepEqual(gotOnlyB, tc.wantOnlyB) {
				t.Errorf("onlyB: got %v, want %v", gotOnlyB, tc.wantOnlyB)
			}
			if !reflect.DeepEqual(gotBoth, tc.wantBoth) {
				t.Errorf("both: got %v, want %v", gotBoth, tc.wantBoth)
			}
		})
	}
}

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
