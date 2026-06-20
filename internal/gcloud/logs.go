package gcloud

import (
	"encoding/json"
	"fmt"

	"github.com/gpnaslund/gcp-tui/internal/run"
)

// LogQuery describes what to fetch from Cloud Logging.
type LogQuery struct {
	Project    string
	DatabaseID string
	Freshness  string
	Severity   string
	Grep       string
	Limit      int
}

// LogEntry is a single Cloud Logging record, flattened for display.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// rawEntry is the Cloud Logging JSON shape returned by gcloud logging read.
type rawEntry struct {
	Timestamp   string          `json:"timestamp"`
	Severity    string          `json:"severity"`
	TextPayload string          `json:"textPayload"`
	JSONPayload json.RawMessage `json:"jsonPayload"`
}

// buildLogFilter returns a Cloud Logging filter for a Cloud SQL instance,
// appending a severity lower-bound and a grep term when set.
func buildLogFilter(databaseID, severity, grep string) string {
	f := fmt.Sprintf(`resource.type="cloudsql_database" AND resource.labels.database_id="%s"`, databaseID)
	if severity != "" {
		f += fmt.Sprintf(" AND severity>=%s", severity)
	}
	if grep != "" {
		f += fmt.Sprintf(" AND %q", grep)
	}
	return f
}

// ReadLogs fetches log entries from Cloud Logging for the given query.
func ReadLogs(q LogQuery) ([]LogEntry, error) {
	filter := buildLogFilter(q.DatabaseID, q.Severity, q.Grep)
	freshness := q.Freshness
	if freshness == "" {
		freshness = "1h"
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	out, err := run.Output(
		"gcloud", "logging", "read", filter,
		"--project", q.Project,
		"--limit", fmt.Sprintf("%d", limit),
		"--freshness", freshness,
		"--order", "desc",
		"--format=json",
	)
	if err != nil {
		return nil, err
	}
	var raws []rawEntry
	if err := json.Unmarshal(out, &raws); err != nil {
		return nil, fmt.Errorf("parse gcloud logging read: %w", err)
	}
	entries := make([]LogEntry, len(raws))
	for i, r := range raws {
		entries[i] = LogEntry{
			Timestamp: r.Timestamp,
			Severity:  r.Severity,
			Message:   flattenPayload(r),
		}
	}
	return entries, nil
}

// flattenPayload derives a message string from the raw entry: textPayload wins,
// then jsonPayload as compact JSON, then empty string.
func flattenPayload(r rawEntry) string {
	if r.TextPayload != "" {
		return r.TextPayload
	}
	if len(r.JSONPayload) > 0 {
		compact, err := json.Marshal(json.RawMessage(r.JSONPayload))
		if err == nil {
			return string(compact)
		}
	}
	return ""
}
