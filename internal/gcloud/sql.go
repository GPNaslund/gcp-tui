package gcloud

import (
	"encoding/json"
	"fmt"

	"github.com/gpnaslund/gcp-tui/internal/run"
)

// Database is a Cloud SQL database within an instance.
type Database struct {
	Name      string `json:"name"`
	Charset   string `json:"charset"`
	Collation string `json:"collation"`
}

// SQLUser is a Cloud SQL user.
type SQLUser struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Type string `json:"type"`
}

// InstanceDetail is the flattened view of a Cloud SQL instance from
// `gcloud sql instances describe`.
type InstanceDetail struct {
	Name             string   `json:"name"`
	DatabaseVersion  string   `json:"databaseVersion"`
	Region           string   `json:"region"`
	State            string   `json:"state"`
	ConnectionName   string   `json:"connectionName"`
	Tier             string   `json:"tier"`
	AvailabilityType string   `json:"availabilityType"`
	DiskSizeGb       string   `json:"diskSizeGb"`
	BackupEnabled    bool     `json:"backupEnabled"`
	IPAddresses      []string `json:"ipAddresses"`
}

// rawInstanceDetail mirrors the nested gcloud JSON shape for `sql instances describe`.
type rawInstanceDetail struct {
	Name            string `json:"name"`
	DatabaseVersion string `json:"databaseVersion"`
	Region          string `json:"region"`
	State           string `json:"state"`
	ConnectionName  string `json:"connectionName"`
	Settings        struct {
		Tier                string `json:"tier"`
		AvailabilityType    string `json:"availabilityType"`
		DataDiskSizeGb      string `json:"dataDiskSizeGb"`
		BackupConfiguration struct {
			Enabled bool `json:"enabled"`
		} `json:"backupConfiguration"`
	} `json:"settings"`
	IPAddresses []struct {
		IPAddress string `json:"ipAddress"`
	} `json:"ipAddresses"`
}

// ListDatabases returns the databases in a Cloud SQL instance.
func ListDatabases(project, instance string) ([]Database, error) {
	out, err := run.Output("gcloud", "sql", "databases", "list",
		"--instance", instance,
		"--project", project,
		"--format=json",
	)
	if err != nil {
		return nil, err
	}
	var dbs []Database
	if err := json.Unmarshal(out, &dbs); err != nil {
		return nil, fmt.Errorf("parse gcloud sql databases list: %w", err)
	}
	return dbs, nil
}

// ListUsers returns the users in a Cloud SQL instance.
func ListUsers(project, instance string) ([]SQLUser, error) {
	out, err := run.Output("gcloud", "sql", "users", "list",
		"--instance", instance,
		"--project", project,
		"--format=json",
	)
	if err != nil {
		return nil, err
	}
	var users []SQLUser
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, fmt.Errorf("parse gcloud sql users list: %w", err)
	}
	return users, nil
}

// DescribeInstance fetches and flattens a Cloud SQL instance description.
func DescribeInstance(project, instance string) (InstanceDetail, error) {
	out, err := run.Output("gcloud", "sql", "instances", "describe", instance,
		"--project", project,
		"--format=json",
	)
	if err != nil {
		return InstanceDetail{}, err
	}
	var raw rawInstanceDetail
	if err := json.Unmarshal(out, &raw); err != nil {
		return InstanceDetail{}, fmt.Errorf("parse gcloud sql instances describe: %w", err)
	}
	return flattenInstance(raw), nil
}

// flattenInstance converts the nested gcloud JSON into the flat InstanceDetail.
func flattenInstance(raw rawInstanceDetail) InstanceDetail {
	addrs := make([]string, len(raw.IPAddresses))
	for i, a := range raw.IPAddresses {
		addrs[i] = a.IPAddress
	}
	return InstanceDetail{
		Name:             raw.Name,
		DatabaseVersion:  raw.DatabaseVersion,
		Region:           raw.Region,
		State:            raw.State,
		ConnectionName:   raw.ConnectionName,
		Tier:             raw.Settings.Tier,
		AvailabilityType: raw.Settings.AvailabilityType,
		DiskSizeGb:       raw.Settings.DataDiskSizeGb,
		BackupEnabled:    raw.Settings.BackupConfiguration.Enabled,
		IPAddresses:      addrs,
	}
}
