package cli

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
	"github.com/gpnaslund/gcp-tui/internal/tui"
)

// runTUI launches the interactive cockpit (bare `gcp-tui`).
func runTUI() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	doc, _ := doctor.Inspect()
	_, err = tea.NewProgram(tui.New(cfg, doc), tea.WithAltScreen()).Run()
	return err
}
