package run

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
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

// TestOutputFoldsStderrIntoError covers the MCP-facing fix: a failing command's
// stderr must travel in the returned error (not vanish to this process's
// stderr), while the error still unwraps to *exec.ExitError.
func TestOutputFoldsStderrIntoError(t *testing.T) {
	_, err := Output("sh", "-c", "echo 'boom-diagnostic' >&2; exit 3")
	if err == nil {
		t.Fatal("expected an error from a failing command")
	}
	if !strings.Contains(err.Error(), "boom-diagnostic") {
		t.Fatalf("error should carry the command's stderr; got: %v", err)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("error should still wrap *exec.ExitError; got %T", err)
	}
}

func TestOutputReturnsStdoutOnSuccess(t *testing.T) {
	out, err := Output("sh", "-c", "printf hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q, want %q", out, "hello")
	}
}
