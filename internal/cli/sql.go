package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/gcloud"
)

func sqlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Cloud SQL read operations scoped to an environment's instance",
	}
	cmd.AddCommand(sqlDatabasesCmd())
	cmd.AddCommand(sqlUsersCmd())
	cmd.AddCommand(sqlDescribeCmd())
	return cmd
}

func sqlDatabasesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "databases <env>",
		Short: "List databases in an environment's Cloud SQL instance",
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
			dbs, err := gcloud.ListDatabases(env.Project, instance)
			if err != nil {
				return err
			}
			return emit(dbs, func() error {
				if len(dbs) == 0 {
					fmt.Fprintln(out, "no databases")
					return nil
				}
				w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tCHARSET\tCOLLATION")
				for _, db := range dbs {
					fmt.Fprintf(w, "%s\t%s\t%s\n", db.Name, db.Charset, db.Collation)
				}
				return w.Flush()
			})
		},
	}
}

func sqlUsersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "users <env>",
		Short: "List users in an environment's Cloud SQL instance",
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
			users, err := gcloud.ListUsers(env.Project, instance)
			if err != nil {
				return err
			}
			return emit(users, func() error {
				if len(users) == 0 {
					fmt.Fprintln(out, "no users")
					return nil
				}
				w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tHOST\tTYPE")
				for _, u := range users {
					fmt.Fprintf(w, "%s\t%s\t%s\n", u.Name, u.Host, u.Type)
				}
				return w.Flush()
			})
		},
	}
}

func sqlDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <env>",
		Short: "Describe the Cloud SQL instance for an environment",
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
			detail, err := gcloud.DescribeInstance(env.Project, instance)
			if err != nil {
				return err
			}
			return emit(detail, func() error {
				pairs := []struct{ k, v string }{
					{"Name", detail.Name},
					{"DatabaseVersion", detail.DatabaseVersion},
					{"Region", detail.Region},
					{"State", detail.State},
					{"ConnectionName", detail.ConnectionName},
					{"Tier", detail.Tier},
					{"AvailabilityType", detail.AvailabilityType},
					{"DiskSizeGb", detail.DiskSizeGb},
					{"BackupEnabled", fmt.Sprintf("%v", detail.BackupEnabled)},
					{"IPAddresses", strings.Join(detail.IPAddresses, ", ")},
				}
				w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				for _, p := range pairs {
					fmt.Fprintf(w, "%s\t%s\n", p.k, p.v)
				}
				return w.Flush()
			})
		},
	}
}
