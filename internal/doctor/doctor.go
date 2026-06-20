// Package doctor checks the prerequisites and authentication state the proxy
// depends on, and offers to run the right gcloud flow when something fixable is
// missing. It never reimplements OAuth — it triggers gcloud's own flows.
package doctor

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/huh"

	"github.com/gpnaslund/gcp-tui/internal/gcloud"
	"github.com/gpnaslund/gcp-tui/internal/run"
)

// Result is a snapshot of the local environment's readiness.
type Result struct {
	GcloudInstalled bool
	ProxyInstalled  bool
	PsqlInstalled   bool
	ActiveAccount   string
	HasAccount      bool
	AccountValid    bool
	HasADC          bool
	ADCValid        bool
}

func have(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// Inspect gathers the current state without changing anything.
func Inspect() (Result, error) {
	r := Result{
		GcloudInstalled: have("gcloud"),
		ProxyInstalled:  have("cloud-sql-proxy"),
		PsqlInstalled:   have("psql"),
	}
	if !r.GcloudInstalled {
		return r, nil
	}
	acc, ok, err := gcloud.ActiveAccount()
	if err != nil {
		return r, err
	}
	r.ActiveAccount, r.HasAccount = acc, ok
	if ok {
		valid, err := gcloud.AccountValid()
		if err != nil {
			return r, err
		}
		r.AccountValid = valid
	}

	adc, err := gcloud.ADCExists()
	if err != nil {
		return r, err
	}
	r.HasADC = adc
	if adc {
		valid, err := gcloud.ADCValid()
		if err != nil {
			return r, err
		}
		r.ADCValid = valid
	}
	return r, nil
}

// Ensure inspects and, for fixable auth gaps, offers to run the matching gcloud
// flow. Missing binaries are reported as errors but never auto-installed — they
// need a package manager the user should drive.
func Ensure(interactive bool) (Result, error) {
	r, err := Inspect()
	if err != nil {
		return r, err
	}
	if !r.GcloudInstalled {
		return r, fmt.Errorf("gcloud not found on PATH; install the Google Cloud CLI first")
	}

	if !r.HasAccount {
		if interactive && confirm("No gcloud account is logged in. Run `gcloud auth login` now?") {
			if err := run.Inherit("gcloud", "auth", "login"); err != nil {
				return r, err
			}
		} else {
			return r, fmt.Errorf("no active gcloud account; run: gcloud auth login")
		}
	} else if !r.AccountValid {
		if interactive && confirm("Your gcloud account is logged in but its credentials are expired or need reauthentication (real gcloud calls will fail). Run `gcloud auth login` now?") {
			if err := run.Inherit("gcloud", "auth", "login"); err != nil {
				return r, err
			}
		} else {
			return r, fmt.Errorf("gcloud account credentials need reauthentication; run: gcloud auth login")
		}
	}

	if !r.HasADC {
		if interactive && confirm("Application Default Credentials are missing (the proxy needs them). Run `gcloud auth application-default login` now?") {
			if err := run.Inherit("gcloud", "auth", "application-default", "login"); err != nil {
				return r, err
			}
		} else {
			return r, fmt.Errorf("no application default credentials; run: gcloud auth application-default login")
		}
	} else if !r.ADCValid {
		if interactive && confirm("Application Default Credentials are expired or need reauthentication (the proxy can't mint a token). Run `gcloud auth application-default login` now?") {
			if err := run.Inherit("gcloud", "auth", "application-default", "login"); err != nil {
				return r, err
			}
		} else {
			return r, fmt.Errorf("application default credentials need reauthentication; run: gcloud auth application-default login")
		}
	}

	return Inspect()
}

func confirm(msg string) bool {
	var ok bool
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(msg).Value(&ok)))
	if err := form.Run(); err != nil {
		return false
	}
	return ok
}
