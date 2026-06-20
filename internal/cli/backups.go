package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/gcloud"
	"github.com/gpnaslund/gcp-tui/internal/run"
)

func backupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Cloud SQL backup operations scoped to an environment's instance",
	}
	cmd.AddCommand(backupsListCmd())
	cmd.AddCommand(backupsCreateCmd())
	return cmd
}

func backupsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <env>",
		Short: "List backups for an environment's Cloud SQL instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			env, ok := cfg.Find(args[0])
			if !ok {
				return fmt.Errorf("environment %q not found", args[0])
			}
			instance := lastColonSegment(env.Instance)
			backups, err := gcloud.ListBackups(env.Project, instance)
			if err != nil {
				return err
			}
			return emit(backups, func() error {
				if len(backups) == 0 {
					fmt.Fprintln(out, "no backups")
					return nil
				}
				w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tWINDOW_START\tSTATUS\tTYPE\tDESCRIPTION")
				for _, b := range backups {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", b.ID, b.WindowStartTime, b.Status, b.Type, b.Description)
				}
				return w.Flush()
			})
		},
	}
}

func backupsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <env>",
		Short: "Create an on-demand backup of an environment's Cloud SQL instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			env, ok := cfg.Find(args[0])
			if !ok {
				return fmt.Errorf("environment %q not found", args[0])
			}
			instance := lastColonSegment(env.Instance)

			// Dry-run: preview the command without mutating anything — no gate needed.
			if run.DryRun {
				return gcloud.CreateBackup(env.Project, instance)
			}

			// Real run: full safety gate before any mutation.
			if err := authorizeWrite(*env); err != nil {
				return err
			}
			return gcloud.CreateBackup(env.Project, instance)
		},
	}
}
