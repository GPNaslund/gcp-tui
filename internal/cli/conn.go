package cli

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/secret"
)

func connCmd() *cobra.Command {
	var doCopy bool
	cmd := &cobra.Command{
		Use:   "conn <env> [profile]",
		Short: "Print a ready-to-use connection string for an environment profile",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			profName := ""
			if len(args) == 2 {
				profName = args[1]
			}
			return printConn(args[0], profName, doCopy)
		},
	}
	cmd.Flags().BoolVar(&doCopy, "copy", false, "copy to the clipboard instead of printing the password to the terminal")
	return cmd
}

func printConn(envName, profName string, doCopy bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	env, ok := cfg.Find(envName)
	if !ok {
		return fmt.Errorf("no environment %q", envName)
	}

	prof, err := resolveProfile(env, profName)
	if err != nil {
		return err
	}

	password := ""
	if !env.IAMAuth {
		pw, err := secret.Get(envName, prof.Name)
		if err != nil {
			return fmt.Errorf("no stored password for %s/%s (%w); re-add it with `gcp-tui profile add %s`", envName, prof.Name, err, envName)
		}
		password = pw
	}

	s := env.ConnString(*prof, password)
	if doCopy {
		if err := clipboard.WriteAll(s); err != nil {
			return fmt.Errorf("could not copy to clipboard (install wl-clipboard or xclip): %w", err)
		}
		fmt.Printf("Connection string for %s/%s copied to clipboard.\n", envName, prof.Name)
		return nil
	}
	fmt.Println(s)
	return nil
}

// resolveProfile selects the requested profile, or the only one when the env
// has exactly one and none was named.
func resolveProfile(env *config.Env, name string) (*config.Profile, error) {
	if name != "" {
		p, ok := env.FindProfile(name)
		if !ok {
			return nil, fmt.Errorf("no profile %q on %q", name, env.Name)
		}
		return p, nil
	}
	switch len(env.Profiles) {
	case 0:
		return nil, fmt.Errorf("no profiles on %q; add one with `gcp-tui profile add %s`", env.Name, env.Name)
	case 1:
		return &env.Profiles[0], nil
	default:
		names := make([]string, 0, len(env.Profiles))
		for _, p := range env.Profiles {
			names = append(names, p.Name)
		}
		return nil, fmt.Errorf("%q has multiple profiles (%s); name one", env.Name, strings.Join(names, ", "))
	}
}
