package gcloud

import (
	"encoding/json"
	"testing"
)

const describeFixture = `{
	"name": "velora-staging",
	"databaseVersion": "POSTGRES_15",
	"region": "us-central1",
	"state": "RUNNABLE",
	"connectionName": "my-project:us-central1:velora-staging",
	"settings": {
		"tier": "db-custom-2-7680",
		"availabilityType": "REGIONAL",
		"dataDiskSizeGb": "100",
		"backupConfiguration": {
			"enabled": true
		}
	},
	"ipAddresses": [
		{"ipAddress": "10.0.0.1", "type": "PRIVATE"},
		{"ipAddress": "34.1.2.3", "type": "PRIMARY"}
	]
}`

func TestFlattenInstance(t *testing.T) {
	var raw rawInstanceDetail
	if err := json.Unmarshal([]byte(describeFixture), &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	d := flattenInstance(raw)

	check := func(field, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %q, want %q", field, got, want)
		}
	}

	check("Name", d.Name, "velora-staging")
	check("DatabaseVersion", d.DatabaseVersion, "POSTGRES_15")
	check("Region", d.Region, "us-central1")
	check("State", d.State, "RUNNABLE")
	check("ConnectionName", d.ConnectionName, "my-project:us-central1:velora-staging")
	check("Tier", d.Tier, "db-custom-2-7680")
	check("AvailabilityType", d.AvailabilityType, "REGIONAL")
	check("DiskSizeGb", d.DiskSizeGb, "100")

	if !d.BackupEnabled {
		t.Error("BackupEnabled: got false, want true")
	}
	if len(d.IPAddresses) != 2 {
		t.Fatalf("IPAddresses: got %d entries, want 2", len(d.IPAddresses))
	}
	if d.IPAddresses[0] != "10.0.0.1" {
		t.Errorf("IPAddresses[0]: got %q, want %q", d.IPAddresses[0], "10.0.0.1")
	}
	if d.IPAddresses[1] != "34.1.2.3" {
		t.Errorf("IPAddresses[1]: got %q, want %q", d.IPAddresses[1], "34.1.2.3")
	}
}
