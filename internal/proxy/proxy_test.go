package proxy

import (
	"reflect"
	"testing"
)

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
