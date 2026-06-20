package gcloud

import (
	"encoding/json"
	"fmt"

	"github.com/gpnaslund/gcp-tui/internal/run"
)

// Backup represents a single Cloud SQL backup entry from `gcloud sql backups list`.
type Backup struct {
	ID              string `json:"id"`
	WindowStartTime string `json:"windowStartTime"`
	Status          string `json:"status"`
	Type            string `json:"type"`
	Description     string `json:"description"`
}

// ListBackups returns the on-demand and scheduled backups for a Cloud SQL instance.
func ListBackups(project, instance string) ([]Backup, error) {
	out, err := run.Output("gcloud", "sql", "backups", "list",
		"--instance", instance,
		"--project", project,
		"--format=json",
	)
	if err != nil {
		return nil, err
	}
	var backups []Backup
	if err := json.Unmarshal(out, &backups); err != nil {
		return nil, fmt.Errorf("parse gcloud sql backups list: %w", err)
	}
	return backups, nil
}

// CreateBackup triggers an on-demand backup of a Cloud SQL instance.
// It uses run.Inherit so gcloud's progress output streams to the terminal and
// --dry-run is honoured by run.Inherit (prints the command and returns ErrDryRun).
func CreateBackup(project, instance string) error {
	return run.Inherit("gcloud", "sql", "backups", "create",
		"--instance", instance,
		"--project", project,
	)
}
