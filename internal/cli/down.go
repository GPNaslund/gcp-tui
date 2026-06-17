package cli

import (
	"fmt"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/proxy"
)

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down <env>",
		Short: "Kill the proxy listening on an env's reserved slot",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDown(args[0])
		},
	}
}

func runDown(envName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	envPtr, ok := cfg.Find(envName)
	if !ok {
		return fmt.Errorf("no environment %q; run `gcp-tui list`", envName)
	}
	env := *envPtr

	pids, err := proxy.ListenerPIDs(env)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return fmt.Errorf("nothing to kill: %s:%d has no listener", env.Address, env.Port)
	}

	confirm := false
	if ferr := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Kill PID %v on %s:%d (%s)? ", pids, env.Address, env.Port, env.Instance)).
			Value(&confirm),
	)).Run(); ferr != nil || !confirm {
		return fmt.Errorf("aborted")
	}

	var errs []error
	for _, pid := range pids {
		if kerr := syscall.Kill(pid, syscall.SIGTERM); kerr != nil {
			errs = append(errs, fmt.Errorf("kill PID %d: %w", pid, kerr))
		} else {
			fmt.Printf("killed PID %d\n", pid)
		}
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Println(e)
		}
		return fmt.Errorf("some kills failed")
	}
	return nil
}
