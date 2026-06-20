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
	// WithFilter installs QuitCleanupFilter, which SIGTERMs every tracked tunnel on
	// any quit — including Bubble Tea's own SIGINT/SIGTERM handling, which exits
	// without routing through Update/handleKey.
	_, err = tea.NewProgram(tui.New(cfg, doc), tea.WithAltScreen(), tea.WithFilter(tui.QuitCleanupFilter)).Run()
	return err
}
