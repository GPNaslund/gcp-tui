// Package gcloud is a thin adapter over the gcloud CLI. It is the only place
// that knows the shape of gcloud's JSON output; everything else consumes these
// typed results.
package gcloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gpnaslund/gcp-tui/internal/run"
)

// Account is one credentialed gcloud account.
type Account struct {
	Account string `json:"account"`
	Status  string `json:"status"`
}

// Project is a GCP project the active account can see.
type Project struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

// Instance is a Cloud SQL instance. ConnectionName is the project:region:instance
// triple the proxy needs.
type Instance struct {
	Name            string `json:"name"`
	ConnectionName  string `json:"connectionName"`
	Region          string `json:"region"`
	DatabaseVersion string `json:"databaseVersion"`
	State           string `json:"state"`
}

// Installed reports whether the gcloud CLI is on PATH.
func Installed() bool {
	_, err := exec.LookPath("gcloud")
	return err == nil
}

// Accounts returns all credentialed gcloud accounts.
func Accounts() ([]Account, error) {
	out, err := run.Output("gcloud", "auth", "list", "--format=json")
	if err != nil {
		return nil, err
	}
	var accs []Account
	if err := json.Unmarshal(out, &accs); err != nil {
		return nil, fmt.Errorf("parse gcloud auth list: %w", err)
	}
	return accs, nil
}

// ActiveAccount returns the active gcloud account, if any.
func ActiveAccount() (string, bool, error) {
	accs, err := Accounts()
	if err != nil {
		return "", false, err
	}
	for _, a := range accs {
		if a.Status == "ACTIVE" {
			return a.Account, true, nil
		}
	}
	return "", false, nil
}

// ADCExists reports whether Application Default Credentials are present. The
// proxy authenticates with ADC, not the gcloud user credential, so this is a
// distinct check.
func ADCExists() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	_, err = os.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

// ADCValid reports whether Application Default Credentials can currently mint an
// access token. ADCExists only proves the credential file is on disk; the
// refresh token it holds expires and is subject to Google's periodic
// reauthentication, which the proxy surfaces at connect time as
// "invalid_grant"/"invalid_rapt". Asking gcloud to print a token forces the
// same refresh the proxy will do, so it is the only reliable probe. A non-zero
// exit means the token can't be minted (expired or reauth required) — that is
// the answer, not an error to propagate.
func ADCValid() (bool, error) {
	if _, err := run.Output("gcloud", "auth", "application-default", "print-access-token"); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListProjects returns the projects the active account can enumerate.
func ListProjects() ([]Project, error) {
	out, err := run.Output("gcloud", "projects", "list", "--format=json")
	if err != nil {
		return nil, err
	}
	var ps []Project
	if err := json.Unmarshal(out, &ps); err != nil {
		return nil, fmt.Errorf("parse gcloud projects list: %w", err)
	}
	return ps, nil
}

// ListInstances returns the Cloud SQL instances in a project.
func ListInstances(project string) ([]Instance, error) {
	out, err := run.Output("gcloud", "sql", "instances", "list", "--project", project, "--format=json")
	if err != nil {
		return nil, err
	}
	var is []Instance
	if err := json.Unmarshal(out, &is); err != nil {
		return nil, fmt.Errorf("parse gcloud sql instances list: %w", err)
	}
	return is, nil
}
