package secretmanager

import "testing"

func TestShortName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"projects/123456/secrets/DB_PASSWORD", "DB_PASSWORD"},
		{"projects/p/secrets/api-key/versions/7", "7"},
		{"NAME", "NAME"},
	}
	for _, c := range cases {
		got := shortName(c.input)
		if got != c.want {
			t.Errorf("shortName(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}
