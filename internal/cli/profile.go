package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/secret"
)

func profileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage connection profiles (user/db/password) attached to an environment",
	}
	cmd.AddCommand(profileAddCmd(), profileListCmd(), profileRemoveCmd())
	return cmd
}

func profileAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <env>",
		Short: "Add a connection profile to an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return addProfile(args[0])
		},
	}
}

func addProfile(envName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	env, ok := cfg.Find(envName)
	if !ok {
		return fmt.Errorf("no environment %q; run `gcp-tui list`", envName)
	}

	p := config.Profile{SSLMode: "disable"}
	var password string

	fields := []huh.Field{
		huh.NewInput().Title("Profile name").Placeholder("app").Value(&p.Name),
		huh.NewInput().Title("Database user").Value(&p.User),
		huh.NewInput().Title("Database name").Value(&p.DBName),
		huh.NewInput().Title("sslmode").Value(&p.SSLMode),
	}
	if !env.IAMAuth {
		fields = append(fields, huh.NewInput().
			Title("Password").
			Description("stored in the OS keyring, never in the config file").
			EchoMode(huh.EchoModePassword).
			Value(&password))
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return err
	}

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if _, exists := env.FindProfile(p.Name); exists {
		return fmt.Errorf("profile %q already exists on %q", p.Name, envName)
	}

	env.Profiles = append(env.Profiles, p)
	if err := cfg.Save(); err != nil {
		return err
	}
	if !env.IAMAuth && password != "" {
		if err := secret.Set(envName, p.Name, password); err != nil {
			return fmt.Errorf("profile saved, but storing the password in the keyring failed: %w", err)
		}
	}
	fmt.Printf("Added profile %q to %q. Get the connection string with: gcp-tui conn %s %s\n", p.Name, envName, envName, p.Name)
	return nil
}

func profileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [env]",
		Short: "List connection profiles",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ENV\tPROFILE\tUSER\tDATABASE")
			for _, e := range cfg.Envs {
				if len(args) == 1 && e.Name != args[0] {
					continue
				}
				for _, p := range e.Profiles {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Name, p.Name, p.User, p.DBName)
				}
			}
			return w.Flush()
		},
	}
}

func profileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <env> <profile>",
		Short: "Remove a connection profile and its stored password",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			envName, profName := args[0], args[1]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			env, ok := cfg.Find(envName)
			if !ok {
				return fmt.Errorf("no environment %q", envName)
			}
			if !env.RemoveProfile(profName) {
				return fmt.Errorf("no profile %q on %q", profName, envName)
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			if err := secret.Delete(envName, profName); err != nil {
				return fmt.Errorf("profile removed, but clearing its keyring password failed: %w", err)
			}
			fmt.Printf("Removed profile %q from %q\n", profName, envName)
			return nil
		},
	}
}
