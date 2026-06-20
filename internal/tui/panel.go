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
