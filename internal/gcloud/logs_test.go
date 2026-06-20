package gcloud

import (
	"encoding/json"
	"testing"
)

func TestBuildLogFilter_basic(t *testing.T) {
	got := buildLogFilter("my-project:my-instance", "", "")
	want := `resource.type="cloudsql_database" AND resource.labels.database_id="my-project:my-instance"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildLogFilter_severity(t *testing.T) {
	got := buildLogFilter("proj:inst", "ERROR", "")
	want := `resource.type="cloudsql_database" AND resource.labels.database_id="proj:inst" AND severity>=ERROR`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildLogFilter_grep(t *testing.T) {
	got := buildLogFilter("proj:inst", "", "connection refused")
	want := `resource.type="cloudsql_database" AND resource.labels.database_id="proj:inst" AND "connection refused"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildLogFilter_severityAndGrep(t *testing.T) {
	got := buildLogFilter("proj:inst", "WARNING", "timeout")
	want := `resource.type="cloudsql_database" AND resource.labels.database_id="proj:inst" AND severity>=WARNING AND "timeout"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFlattenPayload_textPayload(t *testing.T) {
	r := rawEntry{TextPayload: "hello world", Severity: "INFO"}
	if got := flattenPayload(r); got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestFlattenPayload_jsonPayload(t *testing.T) {
	raw := json.RawMessage(`{"msg":"db connected","pid":42}`)
	r := rawEntry{JSONPayload: raw}
	got := flattenPayload(r)
	if got == "" {
		t.Error("expected non-empty message from jsonPayload")
	}
}

func TestFlattenPayload_textPayloadWins(t *testing.T) {
	raw := json.RawMessage(`{"msg":"ignored"}`)
	r := rawEntry{TextPayload: "text wins", JSONPayload: raw}
	if got := flattenPayload(r); got != "text wins" {
		t.Errorf("got %q, want %q", got, "text wins")
	}
}

func TestFlattenPayload_empty(t *testing.T) {
	r := rawEntry{}
	if got := flattenPayload(r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
