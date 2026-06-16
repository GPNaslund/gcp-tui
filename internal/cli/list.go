package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured environments",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Envs) == 0 {
				fmt.Println("No environments configured. Run `gcp-tui init`.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSLOT\tCONFIRM\tINSTANCE")
			for _, e := range cfg.Envs {
				fmt.Fprintf(w, "%s\t%s:%d\t%v\t%s\n", e.Name, e.Address, e.Port, e.Confirm, e.Instance)
			}
			return w.Flush()
		},
	}
}
