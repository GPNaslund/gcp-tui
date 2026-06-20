package proxy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/run"
)

// TestStartDryRun locks in the safety guard: under --dry-run, Start must return
// ErrDryRun without launching the proxy. The guard returns before SlotBusy and
// cmd.Start, so this never touches the network or spawns cloud-sql-proxy.
func TestStartDryRun(t *testing.T) {
	run.DryRun = true
	t.Cleanup(func() { run.DryRun = false })

	e := config.Env{Name: "test", Address: "127.0.0.99", Port: 1, Instance: "p:r:i"}
	if err := Start(e); !errors.Is(err, run.ErrDryRun) {
		t.Fatalf("Start under dry-run = %v; want ErrDryRun", err)
	}
}

func TestParsePIDs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []int
	}{
		{
			name: "multi",
			in:   "123\n456\n",
			want: []int{123, 456},
		},
		{
			name: "single",
			in:   "789",
			want: []int{789},
		},
		{
			name: "empty string",
			in:   "",
			want: nil,
		},
		{
			name: "whitespace only",
			in:   "  \n  \n",
			want: nil,
		},
		{
			name: "leading and trailing whitespace on pid lines",
			in:   "  123  \n  456  \n",
			want: []int{123, 456},
		},
		{
			name: "skip non-numeric lines",
			in:   "123\nnot-a-pid\n456\n",
			want: []int{123, 456},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePIDs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parsePIDs(%q) = %v; want %v", tc.in, got, tc.want)
			}
		})
	}
}
