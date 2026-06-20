package cli

import (
	"testing"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

func TestDatabaseID(t *testing.T) {
	cases := []struct {
		name string
		env  config.Env
		want string
	}{
		{
			name: "standard connection name project:region:instance",
			env: config.Env{
				Project:  "my-project",
				Instance: "my-project:europe-north1:my-instance",
			},
			want: "my-project:my-instance",
		},
		{
			name: "no colon in instance (fallback)",
			env: config.Env{
				Project:  "proj",
				Instance: "bare-instance",
			},
			want: "proj:bare-instance",
		},
		{
			name: "instance field equals project:region:instance with hyphens",
			env: config.Env{
				Project:  "fluted-anthem-413815",
				Instance: "fluted-anthem-413815:europe-north2:velora-staging",
			},
			want: "fluted-anthem-413815:velora-staging",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := databaseID(&tc.env)
			if got != tc.want {
				t.Errorf("databaseID() = %q, want %q", got, tc.want)
			}
		})
	}
}
