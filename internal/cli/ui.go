package cli

import "github.com/charmbracelet/lipgloss"

var (
	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	badStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

func check(ok bool) string {
	if ok {
		return okStyle.Render("✓")
	}
	return badStyle.Render("✗")
}
