package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
)

type envView struct {
	Name     string `json:"name"`
	Project  string `json:"project"`
	Instance string `json:"instance"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Confirm  bool   `json:"confirm"`
	IAMAuth  bool   `json:"iam_auth"`
}

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
				return emit([]envView{}, func() error {
					fmt.Fprintln(out, "No environments configured. Run `gcp-tui init`.")
					return nil
				})
			}
			views := make([]envView, len(cfg.Envs))
			for i, e := range cfg.Envs {
				views[i] = envView{
					Name:     e.Name,
					Project:  e.Project,
					Instance: e.Instance,
					Address:  e.Address,
					Port:     e.Port,
					Confirm:  e.Confirm,
					IAMAuth:  e.IAMAuth,
				}
			}
			return emit(views, func() error {
				w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tSLOT\tCONFIRM\tINSTANCE")
				for _, e := range cfg.Envs {
					fmt.Fprintf(w, "%s\t%s:%d\t%v\t%s\n", e.Name, e.Address, e.Port, e.Confirm, e.Instance)
				}
				return w.Flush()
			})
		},
	}
}
