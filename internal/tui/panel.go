package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/gcloud"
)

// panel is a generic in-cockpit read surface: a titled, scrollable viewport
// filled asynchronously by a tea.Cmd so the gcloud call never blocks Update.
// The open/loading/error/scroll mechanics are surface-agnostic — only the cmd
// that produces a panelDataMsg (e.g. fetchLogsCmd) and its content renderer are
// specific to a given data source.
type panel struct {
	open    bool
	loading bool
	title   string
	err     error
	vp      viewport.Model
}

// panelDataMsg is the result of an async fetch: rendered content for the
// viewport, or err when the fetch failed. title lets the message label the panel
// it fills.
type panelDataMsg struct {
	title   string
	content string
	err     error
}

// fetchLogsCmd reads recent Cloud SQL logs for an env off the Update loop,
// returning a panelDataMsg the model applies to the panel.
func fetchLogsCmd(e config.Env) tea.Cmd {
	return func() tea.Msg {
		entries, err := gcloud.ReadLogs(gcloud.LogQuery{
			Project:    e.Project,
			DatabaseID: e.DatabaseID(),
			Freshness:  "1h",
			Limit:      50,
		})
		if err != nil {
			return panelDataMsg{title: "logs: " + e.Name, err: err}
		}
		return panelDataMsg{title: "logs: " + e.Name, content: renderLogEntries(entries)}
	}
}

// renderLogEntries formats log entries one per line as "timestamp severity
// message", severity-coloured. Empty when there are no entries.
func renderLogEntries(entries []gcloud.LogEntry) string {
	if len(entries) == 0 {
		return lipgloss.NewStyle().Foreground(muted).Render("no log entries in the last hour")
	}
	rows := make([]string, len(entries))
	for i, e := range entries {
		ts := lipgloss.NewStyle().Foreground(muted).Render(e.Timestamp)
		sev := lipgloss.NewStyle().Foreground(severityColor(e.Severity)).Render(fmt.Sprintf("%-8s", e.Severity))
		rows[i] = ts + "  " + sev + "  " + e.Message
	}
	return strings.Join(rows, "\n")
}

// fetchDatabasesCmd lists Cloud SQL databases for an env off the Update loop,
// returning a panelDataMsg the model applies to the panel.
func fetchDatabasesCmd(e config.Env) tea.Cmd {
	return func() tea.Msg {
		dbs, err := gcloud.ListDatabases(e.Project, e.InstanceName())
		if err != nil {
			return panelDataMsg{title: "databases: " + e.Name, err: err}
		}
		return panelDataMsg{title: "databases: " + e.Name, content: renderDatabases(dbs)}
	}
}

// fetchUsersCmd lists Cloud SQL users for an env off the Update loop,
// returning a panelDataMsg the model applies to the panel.
func fetchUsersCmd(e config.Env) tea.Cmd {
	return func() tea.Msg {
		users, err := gcloud.ListUsers(e.Project, e.InstanceName())
		if err != nil {
			return panelDataMsg{title: "users: " + e.Name, err: err}
		}
		return panelDataMsg{title: "users: " + e.Name, content: renderUsers(users)}
	}
}

// fetchDescribeCmd fetches instance detail for an env off the Update loop,
// returning a panelDataMsg the model applies to the panel.
func fetchDescribeCmd(e config.Env) tea.Cmd {
	return func() tea.Msg {
		detail, err := gcloud.DescribeInstance(e.Project, e.InstanceName())
		if err != nil {
			return panelDataMsg{title: "instance: " + e.Name, err: err}
		}
		return panelDataMsg{title: "instance: " + e.Name, content: renderInstanceDetail(detail)}
	}
}

// fetchBackupsCmd lists Cloud SQL backups for an env off the Update loop,
// returning a panelDataMsg the model applies to the panel.
func fetchBackupsCmd(e config.Env) tea.Cmd {
	return func() tea.Msg {
		backups, err := gcloud.ListBackups(e.Project, e.InstanceName())
		if err != nil {
			return panelDataMsg{title: "backups: " + e.Name, err: err}
		}
		return panelDataMsg{title: "backups: " + e.Name, content: renderBackups(backups)}
	}
}

// renderDatabases formats database entries one per line as "name charset collation".
func renderDatabases(dbs []gcloud.Database) string {
	if len(dbs) == 0 {
		return lipgloss.NewStyle().Foreground(muted).Render("no databases found")
	}
	rows := make([]string, len(dbs))
	for i, db := range dbs {
		name := lipgloss.NewStyle().Foreground(cool).Render(fmt.Sprintf("%-30s", db.Name))
		charset := lipgloss.NewStyle().Foreground(ink).Render(fmt.Sprintf("%-12s", db.Charset))
		coll := lipgloss.NewStyle().Foreground(muted).Render(db.Collation)
		rows[i] = name + "  " + charset + "  " + coll
	}
	return strings.Join(rows, "\n")
}

// renderUsers formats SQL user entries one per line as "name host type".
func renderUsers(users []gcloud.SQLUser) string {
	if len(users) == 0 {
		return lipgloss.NewStyle().Foreground(muted).Render("no users found")
	}
	rows := make([]string, len(users))
	for i, u := range users {
		name := lipgloss.NewStyle().Foreground(cool).Render(fmt.Sprintf("%-30s", u.Name))
		host := lipgloss.NewStyle().Foreground(ink).Render(fmt.Sprintf("%-20s", u.Host))
		typ := lipgloss.NewStyle().Foreground(muted).Render(u.Type)
		rows[i] = name + "  " + host + "  " + typ
	}
	return strings.Join(rows, "\n")
}

// renderInstanceDetail formats a labeled key/value block for an InstanceDetail.
func renderInstanceDetail(d gcloud.InstanceDetail) string {
	kv := func(k, v string) string {
		return "  " + label(k) + "  " + lipgloss.NewStyle().Foreground(ink).Render(v)
	}
	backupStr := "disabled"
	if d.BackupEnabled {
		backupStr = "enabled"
	}
	ipStr := strings.Join(d.IPAddresses, ", ")
	if ipStr == "" {
		ipStr = "—"
	}
	diskStr := "—"
	if d.DiskSizeGb != "" {
		diskStr = d.DiskSizeGb + " GiB"
	}
	lines := []string{
		kv("name", d.Name),
		kv("version", d.DatabaseVersion),
		kv("region", d.Region),
		kv("state", d.State),
		kv("tier", d.Tier),
		kv("ha", d.AvailabilityType),
		kv("disk", diskStr),
		kv("backup", backupStr),
		kv("conn", d.ConnectionName),
		kv("ip", ipStr),
	}
	return strings.Join(lines, "\n")
}

// renderBackups formats backup entries one per line as "id time status type".
func renderBackups(backups []gcloud.Backup) string {
	if len(backups) == 0 {
		return lipgloss.NewStyle().Foreground(muted).Render("no backups found")
	}
	rows := make([]string, len(backups))
	for i, b := range backups {
		id := lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf("%-12s", b.ID))
		ts := lipgloss.NewStyle().Foreground(ink).Render(fmt.Sprintf("%-30s", b.WindowStartTime))
		status := lipgloss.NewStyle().Foreground(severityColor(statusSeverity(b.Status))).Render(fmt.Sprintf("%-12s", b.Status))
		typ := lipgloss.NewStyle().Foreground(muted).Render(b.Type)
		rows[i] = id + "  " + ts + "  " + status + "  " + typ
	}
	return strings.Join(rows, "\n")
}

// statusSeverity maps a backup status to a severity string so we can reuse severityColor.
func statusSeverity(status string) string {
	switch status {
	case "FAILED":
		return "ERROR"
	case "RUNNING":
		return "WARNING"
	default:
		return "DEFAULT"
	}
}

// severityColor maps a Cloud Logging severity to the cockpit palette.
func severityColor(severity string) lipgloss.Color {
	switch severity {
	case "ERROR", "CRITICAL", "ALERT", "EMERGENCY":
		return hot
	case "WARNING":
		return amber
	default:
		return ink
	}
}

// renderPanel draws the panel body for the right pane: title rule, then a
// loading line, the error in hot, or the scrollable viewport.
func (p panel) renderPanel(width int) string {
	title := lipgloss.NewStyle().Foreground(cool).Bold(true).Render(p.title)
	rule := lipgloss.NewStyle().Foreground(line).Render(strings.Repeat("─", width))
	var body string
	switch {
	case p.loading:
		body = lipgloss.NewStyle().Foreground(muted).Render("loading…")
	case p.err != nil:
		body = lipgloss.NewStyle().Foreground(hot).Render(p.err.Error())
	default:
		body = p.vp.View()
	}
	return strings.Join([]string{title, rule, body}, "\n")
}
