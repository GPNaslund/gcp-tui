// Package cli wires the cobra command tree.
package cli

import (
	"errors"

	"github.com/gpnaslund/gcp-tui/internal/run"
	"github.com/spf13/cobra"
)

// Execute runs the root command.
func Execute() error {
	root := &cobra.Command{
		Use:   "gcp-tui",
		Short: "Safe, transparent Cloud SQL Auth Proxy launcher for multiple GCP environments",
		Long: "gcp-tui discovers Cloud SQL instances via gcloud, stores them as declared\n" +
			"environments each bound to a distinct reserved loopback slot, and launches the\n" +
			"Cloud SQL Auth Proxy with auth checks, prod confirmation, and full command\n" +
			"transparency.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			run.DryRun = flagDryRun
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI()
		},
	}
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")
	root.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "print the gcloud commands that would run, run nothing")
	root.PersistentFlags().BoolVar(&flagYes, "yes", false, "assume yes for non-prod write confirmations")
	root.AddCommand(initCmd(), doctorCmd(), upCmd(), downCmd(), listCmd(), profileCmd(), connCmd(), secretsCmd())
	err := root.Execute()
	if errors.Is(err, run.ErrDryRun) {
		return nil
	}
	return err
}
