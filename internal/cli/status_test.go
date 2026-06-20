package cli

import (
	"testing"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
)

func TestBuildStatus(t *testing.T) {
	doc := doctor.Result{
		GcloudInstalled: true,
		ProxyInstalled:  true,
		PsqlInstalled:   false,
		ActiveAccount:   "user@example.com",
		HasAccount:      true,
		HasADC:          true,
		ADCValid:        true,
	}
	cfg := &config.Config{
		Envs: []config.Env{
			{
				Name:     "staging",
				Project:  "my-project",
				Instance: "my-project:us-central1:my-db",
				Address:  "127.0.0.2",
				Port:     15433,
				Confirm:  false,
				IAMAuth:  true,
			},
		},
	}

	v := buildStatus(doc, cfg)

	if v.Account != "user@example.com" {
		t.Errorf("Account: got %q, want %q", v.Account, "user@example.com")
	}
	if !v.HasAccount {
		t.Error("HasAccount: got false, want true")
	}
	if !v.ADC.Present {
		t.Error("ADC.Present: got false, want true")
	}
	if !v.ADC.Valid {
		t.Error("ADC.Valid: got false, want true")
	}
	if !v.Tools.Gcloud {
		t.Error("Tools.Gcloud: got false, want true")
	}
	if !v.Tools.Proxy {
		t.Error("Tools.Proxy: got false, want true")
	}
	if v.Tools.Psql {
		t.Error("Tools.Psql: got true, want false")
	}
	if len(v.Environments) != 1 {
		t.Fatalf("Environments: got %d, want 1", len(v.Environments))
	}
	e := v.Environments[0]
	if e.Name != "staging" {
		t.Errorf("env Name: got %q, want %q", e.Name, "staging")
	}
	if e.Project != "my-project" {
		t.Errorf("env Project: got %q, want %q", e.Project, "my-project")
	}
	if e.Instance != "my-project:us-central1:my-db" {
		t.Errorf("env Instance: got %q, want %q", e.Instance, "my-project:us-central1:my-db")
	}
	if e.Address != "127.0.0.2" {
		t.Errorf("env Address: got %q, want %q", e.Address, "127.0.0.2")
	}
	if e.Port != 15433 {
		t.Errorf("env Port: got %d, want %d", e.Port, 15433)
	}
	if e.Confirm {
		t.Error("env Confirm: got true, want false")
	}
	if !e.IAMAuth {
		t.Error("env IAMAuth: got false, want true")
	}
}

func TestBuildStatusNoAccount(t *testing.T) {
	doc := doctor.Result{
		GcloudInstalled: true,
		HasAccount:      false,
		HasADC:          false,
		ADCValid:        false,
	}
	cfg := &config.Config{}

	v := buildStatus(doc, cfg)

	if v.HasAccount {
		t.Error("HasAccount: got true, want false")
	}
	if v.Account != "" {
		t.Errorf("Account: got %q, want empty", v.Account)
	}
	if v.ADC.Present {
		t.Error("ADC.Present: got true, want false")
	}
	if v.ADC.Valid {
		t.Error("ADC.Valid: got true, want false")
	}
	if len(v.Environments) != 0 {
		t.Errorf("Environments: got %d, want 0", len(v.Environments))
	}
}
