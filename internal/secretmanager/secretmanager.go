// Package secretmanager is a thin adapter over the gcloud CLI for Secret
// Manager. It is the only place that knows the shape of gcloud's secrets JSON
// output; everything else consumes these typed results.
package secretmanager

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gpnaslund/gcp-tui/internal/run"
)

// Secret is a Secret Manager secret. Name is the short name (no resource path
// prefix).
type Secret struct {
	Name string
}

// shortName returns the trailing "/"-delimited segment of a resource name.
// If the resource contains no "/", it is returned unchanged.
func shortName(resource string) string {
	if i := strings.LastIndex(resource, "/"); i >= 0 {
		return resource[i+1:]
	}
	return resource
}

// List returns the secrets in project (short names only).
func List(project string) ([]Secret, error) {
	out, err := run.Output("gcloud", "secrets", "list", "--project", project, "--format=json")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gcloud secrets list: %w", err)
	}
	secrets := make([]Secret, len(raw))
	for i, r := range raw {
		secrets[i] = Secret{Name: shortName(r.Name)}
	}
	return secrets, nil
}

// Access returns the latest version value of the named secret in project.
// The value is returned exactly as gcloud emits it — no trimming — because
// secret values may be multiline or end in significant whitespace.
func Access(project, name string) (string, error) {
	out, err := run.Output("gcloud", "secrets", "versions", "access", "latest",
		"--secret", name, "--project", project)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Exists reports whether name is present in project's secret list. On error it
// returns false so callers fail closed.
func Exists(project, name string) (bool, error) {
	secrets, err := List(project)
	if err != nil {
		return false, err
	}
	for _, s := range secrets {
		if s.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Create creates a new secret in project with automatic replication.
func Create(project, name string) error {
	_, err := run.Output("gcloud", "secrets", "create", name,
		"--project", project, "--replication-policy=automatic")
	return err
}

// AddVersion adds a new version of the named secret in project, piping value
// via stdin so the payload is never exposed in the process argument list.
// It returns the short version identifier (e.g. "3").
func AddVersion(project, name string, value []byte) (string, error) {
	out, err := run.OutputInput(value, "gcloud", "secrets", "versions", "add", name,
		"--project", project, "--data-file=-", "--format=value(name)")
	if err != nil {
		return "", err
	}
	return shortName(strings.TrimSpace(string(out))), nil
}
