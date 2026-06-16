// Package cli wires the cobra command tree.
package cli

import "github.com/spf13/cobra"

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
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI()
		},
	}
	root.AddCommand(initCmd(), doctorCmd(), upCmd(), listCmd(), profileCmd(), connCmd())
	return root.Execute()
}
