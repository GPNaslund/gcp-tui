package run

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func setupDryRun(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	DryRun = true
	dryRunOut = buf
	t.Cleanup(func() {
		DryRun = false
		dryRunOut = os.Stdout
	})
	return buf
}

func TestOutputDryRun(t *testing.T) {
	buf := setupDryRun(t)

	out, err := Output("gcloud", "projects", "list")

	if !errors.Is(err, ErrDryRun) {
		t.Fatalf("expected ErrDryRun, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output, got %q", out)
	}
	printed := buf.String()
	if !strings.Contains(printed, "gcloud projects list") {
		t.Fatalf("expected command in output, got %q", printed)
	}
}

func TestOutputInputDryRun(t *testing.T) {
	buf := setupDryRun(t)

	secret := []byte("super-secret-value")
	out, err := OutputInput(secret, "gcloud", "secrets", "versions", "add", "my-secret")

	if !errors.Is(err, ErrDryRun) {
		t.Fatalf("expected ErrDryRun, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output, got %q", out)
	}
	printed := buf.String()
	if !strings.Contains(printed, "gcloud secrets versions add my-secret") {
		t.Fatalf("expected command in output, got %q", printed)
	}
	if strings.Contains(printed, "super-secret-value") {
		t.Fatalf("dry-run output must NOT contain the input bytes, got %q", printed)
	}
}
