package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
	"github.com/gpnaslund/gcp-tui/internal/proxy"
	"github.com/gpnaslund/gcp-tui/internal/secret"
)

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up <env>",
		Short: "Start the Cloud SQL proxy for a configured environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			env, ok := cfg.Find(args[0])
			if !ok {
				return fmt.Errorf("no environment %q; run `gcp-tui list` or `gcp-tui init`", args[0])
			}
			if _, err := doctor.Ensure(true); err != nil {
				return err
			}
			if _, err := exec.LookPath("cloud-sql-proxy"); err != nil {
				return fmt.Errorf("cloud-sql-proxy not found on PATH; install it first")
			}
			if env.Confirm && !typedConfirm(env.Name) {
				return fmt.Errorf("aborted: confirmation did not match %q", env.Name)
			}
			offerConnString(env)
			return proxy.Start(*env)
		},
	}
}

// typedConfirm requires the operator to type the env name before a protected
// tunnel starts.
func typedConfirm(name string) bool {
	fmt.Printf("PROTECTED environment. Type %q to continue: ", name)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line) == name
}

// offerConnString asks, when the env has profiles and we're attached to a
// terminal, whether to print a connection string before the (blocking) tunnel
// starts. On a non-terminal (scripted use) it does nothing.
func offerConnString(env *config.Env) {
	if len(env.Profiles) == 0 || !isatty.IsTerminal(os.Stdin.Fd()) {
		return
	}
	yes := true
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Print a connection string before starting the tunnel?").Value(&yes),
	)).Run(); err != nil || !yes {
		return
	}
	prof, err := pickProfile(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skipping connection string:", err)
		return
	}
	password := ""
	if !env.IAMAuth {
		pw, err := secret.Get(env.Name, prof.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping connection string: no stored password for %s/%s\n", env.Name, prof.Name)
			return
		}
		password = pw
	}
	fmt.Println(env.ConnString(*prof, password))
}

// pickProfile returns the env's only profile, or prompts to choose when there
// are several.
func pickProfile(env *config.Env) (*config.Profile, error) {
	if len(env.Profiles) == 1 {
		return &env.Profiles[0], nil
	}
	opts := make([]huh.Option[string], 0, len(env.Profiles))
	for _, p := range env.Profiles {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s (%s)", p.Name, p.User), p.Name))
	}
	var chosen string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Profile").Options(opts...).Value(&chosen),
	)).Run(); err != nil {
		return nil, err
	}
	p, ok := env.FindProfile(chosen)
	if !ok {
		return nil, fmt.Errorf("profile %q not found", chosen)
	}
	return p, nil
}
