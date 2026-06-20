package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/gcloud"
)

// databaseID derives the Cloud Logging database_id label from an env.
// The connection name is project:region:instance; the label is project:instance.
func databaseID(env *config.Env) string {
	return env.Project + ":" + lastColonSegment(env.Instance)
}

// lastColonSegment returns the portion after the last colon, or the whole
// string if there is no colon.
func lastColonSegment(s string) string {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s
	}
	return s[i+1:]
}

func logsCmd() *cobra.Command {
	var since, severity, grep string
	var limit int

	cmd := &cobra.Command{
		Use:   "logs <env>",
		Short: "Tail Cloud Logging entries for an environment's Cloud SQL instance",
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
			entries, err := gcloud.ReadLogs(gcloud.LogQuery{
				Project:    env.Project,
				DatabaseID: databaseID(env),
				Freshness:  since,
				Severity:   strings.ToUpper(severity),
				Grep:       grep,
				Limit:      limit,
			})
			if err != nil {
				return err
			}
			return emit(entries, func() error {
				if len(entries) == 0 {
					fmt.Fprintln(out, "no log entries")
					return nil
				}
				w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				fmt.Fprintln(w, "TIMESTAMP\tSEVERITY\tMESSAGE")
				for _, e := range entries {
					fmt.Fprintf(w, "%s\t%s\t%s\n", e.Timestamp, e.Severity, e.Message)
				}
				return w.Flush()
			})
		},
	}

	cmd.Flags().StringVar(&since, "since", "1h", "how far back to fetch logs (gcloud freshness format, e.g. 1h, 30m)")
	cmd.Flags().StringVar(&severity, "severity", "", "minimum severity (e.g. ERROR, WARNING, INFO)")
	cmd.Flags().StringVar(&grep, "grep", "", "text to search for in log entries")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of log entries to return")

	return cmd
}
