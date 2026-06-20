package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
)

type adcStatus struct {
	Present bool `json:"present"`
	Valid   bool `json:"valid"`
}

type toolStatus struct {
	Gcloud bool `json:"gcloud"`
	Proxy  bool `json:"proxy"`
	Psql   bool `json:"psql"`
}

type statusView struct {
	Account      string     `json:"account"`
	HasAccount   bool       `json:"has_account"`
	ADC          adcStatus  `json:"adc"`
	Tools        toolStatus `json:"tools"`
	Environments []envView  `json:"environments"`
}

func buildStatus(doc doctor.Result, cfg *config.Config) statusView {
	envs := make([]envView, len(cfg.Envs))
	for i, e := range cfg.Envs {
		envs[i] = envView{
			Name:     e.Name,
			Project:  e.Project,
			Instance: e.Instance,
			Address:  e.Address,
			Port:     e.Port,
			Confirm:  e.Confirm,
			IAMAuth:  e.IAMAuth,
		}
	}
	return statusView{
		Account:    doc.ActiveAccount,
		HasAccount: doc.HasAccount,
		ADC: adcStatus{
			Present: doc.HasADC,
			Valid:   doc.ADCValid,
		},
		Tools: toolStatus{
			Gcloud: doc.GcloudInstalled,
			Proxy:  doc.ProxyInstalled,
			Psql:   doc.PsqlInstalled,
		},
		Environments: envs,
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show account, ADC, tool, and environment status",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			doc, err := doctor.Inspect()
			if err != nil {
				return err
			}
			view := buildStatus(doc, cfg)
			return emit(view, func() error {
				fmt.Fprintln(out)
				if view.HasAccount {
					fmt.Fprintln(out, check(true), "account:", view.Account)
				} else {
					fmt.Fprintln(out, check(false), "no active gcloud account")
				}
				fmt.Fprintln(out, check(view.ADC.Present), "ADC present")
				fmt.Fprintln(out, check(view.ADC.Valid), "ADC valid")
				fmt.Fprintln(out)
				fmt.Fprintln(out, check(view.Tools.Gcloud), "gcloud")
				fmt.Fprintln(out, check(view.Tools.Proxy), "cloud-sql-proxy")
				fmt.Fprintln(out, check(view.Tools.Psql), "psql")
				if len(view.Environments) > 0 {
					fmt.Fprintln(out)
					w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
					fmt.Fprintln(w, "ENV\tPROJECT\tINSTANCE\tSLOT")
					for _, e := range view.Environments {
						fmt.Fprintf(w, "%s\t%s\t%s\t%s:%d\n", e.Name, e.Project, e.Instance, e.Address, e.Port)
					}
					return w.Flush()
				}
				return nil
			})
		},
	}
}
