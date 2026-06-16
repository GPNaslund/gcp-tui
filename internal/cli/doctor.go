package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/doctor"
)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check prerequisites and auth; offer to fix what's missing",
		RunE: func(_ *cobra.Command, _ []string) error {
			r, err := doctor.Ensure(true)
			if err != nil {
				return err
			}
			printDoctor(r)
			return nil
		},
	}
}

func printDoctor(r doctor.Result) {
	fmt.Println()
	fmt.Println(check(r.GcloudInstalled), "gcloud CLI")
	fmt.Println(check(r.ProxyInstalled), "cloud-sql-proxy")
	fmt.Println(check(r.PsqlInstalled), "psql (optional)")
	if r.HasAccount {
		fmt.Println(check(true), "gcloud account:", r.ActiveAccount)
	} else {
		fmt.Println(check(false), "no active gcloud account")
	}
	fmt.Println(check(r.HasADC), "application default credentials (ADC)")
}
