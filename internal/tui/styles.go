package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

// Instrument-panel palette: neutral ink on the terminal's own background, with a
// teal/red/amber accent triad. Teal reads as safe (staging), red as danger
// (prod) — the chrome's temperature is the warning.
var (
	ink   = lipgloss.Color("#E6EDF3")
	muted = lipgloss.Color("#6E7681")
	line  = lipgloss.Color("#30363D")
	cool  = lipgloss.Color("#2DD4BF")
	hot   = lipgloss.Color("#F2555A")
	amber = lipgloss.Color("#E3B341")
)

// riskColor is the signature: cool for safe environments, hot for ones that
// require confirmation (prod).
func riskColor(e config.Env) lipgloss.Color {
	if e.Confirm {
		return hot
	}
	return cool
}

func riskTag(e config.Env) string {
	tag, c := "STAGING", cool
	if e.Confirm {
		tag, c = "PROD", hot
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0B0E14")).
		Background(c).
		Bold(true).
		Padding(0, 1).
		Render(tag)
}

func label(s string) string {
	return lipgloss.NewStyle().Foreground(muted).Render(s)
}

func statusPill(name string, ok bool) string {
	c := cool
	if !ok {
		c = hot
	}
	return lipgloss.NewStyle().Foreground(c).Render("●") + lipgloss.NewStyle().Foreground(muted).Render(name)
}

// fitRow places left and right text on one line of the given width, pushing
// right to the far edge.
func fitRow(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncate(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// pane wraps body in a rounded border of the given color, sized so the bordered
// box is exactly totalW x totalH.
func pane(body string, totalW, totalH int, border lipgloss.Color) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(totalW - 2).
		Height(totalH - 2).
		Padding(0, 1).
		Render(body)
}
