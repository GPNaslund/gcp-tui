package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestEmitJSON(t *testing.T) {
	var buf bytes.Buffer
	out = &buf
	flagJSON = true
	t.Cleanup(func() {
		flagJSON = false
		out = os.Stdout
	})

	type item struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}
	data := []item{{Name: "staging", Port: 15433}}

	if err := emit(data, func() error {
		t.Fatal("human() called when --json is set")
		return nil
	}); err != nil {
		t.Fatalf("emit returned error: %v", err)
	}

	var got []item
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(got) != 1 || got[0].Name != "staging" || got[0].Port != 15433 {
		t.Fatalf("unexpected decoded value: %+v", got)
	}
}

func TestEmitHuman(t *testing.T) {
	var buf bytes.Buffer
	out = &buf
	flagJSON = false
	t.Cleanup(func() {
		out = os.Stdout
	})

	called := false
	if err := emit(nil, func() error {
		called = true
		_, err := buf.WriteString("NAME\tSLOT\tCONFIRM\tINSTANCE\n")
		return err
	}); err != nil {
		t.Fatalf("emit returned error: %v", err)
	}

	if !called {
		t.Fatal("human() was not called")
	}
	if !strings.Contains(buf.String(), "NAME") {
		t.Fatalf("expected table header in output, got: %q", buf.String())
	}
}
