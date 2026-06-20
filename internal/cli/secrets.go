package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
	"github.com/gpnaslund/gcp-tui/internal/secretmanager"
)

func secretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Secret Manager operations scoped to an environment's project",
	}
	cmd.AddCommand(secretsPullCmd())
	cmd.AddCommand(secretsDiffCmd())
	cmd.AddCommand(secretsSetCmd())
	return cmd
}

func secretsSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <env> <secret-name>",
		Short: "Add a new version of a secret, creating it if missing",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runSet(args[0], args[1])
		},
	}
}

func runSet(envName, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	env, ok := cfg.Find(envName)
	if !ok {
		return fmt.Errorf("no environment %q; run `gcp-tui list`", envName)
	}

	if _, err := doctor.Ensure(true); err != nil {
		return err
	}

	exists, err := secretmanager.Exists(env.Project, name)
	if err != nil {
		return err
	}

	// PROD GATE: must run before any Create or AddVersion.
	fmt.Printf("About to set secret %q in project %s.\n", name, env.Project)
	if err := authorizeWrite(*env); err != nil {
		return err
	}

	if !exists {
		create := false
		if ferr := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Secret %q not found in %s. Create it?", name, env.Project)).
				Value(&create),
		)).Run(); ferr != nil || !create {
			return fmt.Errorf("aborted")
		}
		if err := secretmanager.Create(env.Project, name); err != nil {
			return err
		}
	}

	var value string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Secret value").
			EchoMode(huh.EchoModePassword).
			Value(&value),
	)).Run(); err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("aborted: empty value")
	}

	version, err := secretmanager.AddVersion(env.Project, name, []byte(value))
	if err != nil {
		return err
	}
	fmt.Printf("Added version %s of %s in %s\n", version, name, env.Project)
	return nil
}

func secretsDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <env-a> <env-b>",
		Short: "Compare secret names between two environments",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDiff(args[0], args[1])
		},
	}
}

func runDiff(envAName, envBName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	envA, ok := cfg.Find(envAName)
	if !ok {
		return fmt.Errorf("no environment %q; run `gcp-tui list`", envAName)
	}
	envB, ok := cfg.Find(envBName)
	if !ok {
		return fmt.Errorf("no environment %q; run `gcp-tui list`", envBName)
	}

	if _, err := doctor.Ensure(true); err != nil {
		return err
	}

	aSecrets, err := secretmanager.List(envA.Project)
	if err != nil {
		return err
	}
	bSecrets, err := secretmanager.List(envB.Project)
	if err != nil {
		return err
	}

	aNames := make([]string, len(aSecrets))
	for i, s := range aSecrets {
		aNames[i] = s.Name
	}
	bNames := make([]string, len(bSecrets))
	for i, s := range bSecrets {
		bNames[i] = s.Name
	}

	onlyA, onlyB, both := diffNames(aNames, bNames)

	fmt.Printf("Only in %s (%d):\n", envA.Name, len(onlyA))
	for _, n := range onlyA {
		fmt.Printf("  %s\n", n)
	}
	fmt.Printf("Only in %s (%d):\n", envB.Name, len(onlyB))
	for _, n := range onlyB {
		fmt.Printf("  %s\n", n)
	}
	fmt.Printf("In both (%d):\n", len(both))
	for _, n := range both {
		fmt.Printf("  %s\n", n)
	}
	return nil
}

// diffNames performs a pure set-compare of two name lists.
// It returns onlyA (names only in a), onlyB (names only in b), and both
// (names present in both). Each returned slice is sorted ascending and deduped.
func diffNames(a, b []string) (onlyA, onlyB, both []string) {
	setA := make(map[string]bool, len(a))
	for _, n := range a {
		setA[n] = true
	}
	setB := make(map[string]bool, len(b))
	for _, n := range b {
		setB[n] = true
	}

	union := make(map[string]bool, len(setA)+len(setB))
	for n := range setA {
		union[n] = true
	}
	for n := range setB {
		union[n] = true
	}

	for n := range union {
		inA, inB := setA[n], setB[n]
		switch {
		case inA && inB:
			both = append(both, n)
		case inA:
			onlyA = append(onlyA, n)
		default:
			onlyB = append(onlyB, n)
		}
	}

	sort.Strings(onlyA)
	sort.Strings(onlyB)
	sort.Strings(both)
	return onlyA, onlyB, both
}

func secretsPullCmd() *cobra.Command {
	var out string
	var force bool
	cmd := &cobra.Command{
		Use:   "pull <env>",
		Short: "Pull secrets from Secret Manager and write them to a .env file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runPull(args[0], out, force)
		},
	}
	cmd.Flags().StringVar(&out, "out", ".env", "output file path")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite output file without prompting")
	return cmd
}

func runPull(envName, out string, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	env, ok := cfg.Find(envName)
	if !ok {
		return fmt.Errorf("no environment %q; run `gcp-tui list`", envName)
	}

	if _, err := doctor.Ensure(true); err != nil {
		return err
	}

	if env.Confirm && !typedConfirm(env.Name) {
		return fmt.Errorf("aborted: confirmation did not match %q", env.Name)
	}

	secrets, err := secretmanager.List(env.Project)
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		fmt.Printf("no secrets in %s\n", env.Project)
		return nil
	}

	names := make([]string, len(secrets))
	for i, s := range secrets {
		names[i] = s.Name
	}

	selected, err := pickSecrets(names)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		fmt.Println("Nothing selected.")
		return nil
	}

	values := make(map[string]string, len(selected))
	for _, name := range selected {
		v, err := secretmanager.Access(env.Project, name)
		if err != nil {
			return err
		}
		values[name] = v
	}

	if _, err := os.Stat(out); err == nil && !force {
		overwrite := false
		if ferr := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Overwrite %s?", out)).
				Value(&overwrite),
		)).Run(); ferr != nil || !overwrite {
			return fmt.Errorf("aborted")
		}
	}

	gitignoreWarn(out)

	if err := os.WriteFile(out, []byte(formatEnv(values)), 0o600); err != nil {
		return err
	}
	fmt.Printf("Wrote %d secret(s) to %s\n", len(selected), out)
	return nil
}

func pickSecrets(names []string) ([]string, error) {
	opts := make([]huh.Option[string], 0, len(names))
	for _, n := range names {
		opts = append(opts, huh.NewOption(n, n))
	}

	var picked []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Secrets to export").
			Description("Space to toggle, Enter to confirm").
			Options(opts...).
			Value(&picked),
	))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return picked, nil
}

// formatEnv renders name->value as dotenv text. Keys are sorted; values are
// escaped so that backslash, double-quote, and newline are safe inside the
// double-quoted form. Keys are written verbatim.
func formatEnv(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	esc := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n")

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(esc.Replace(values[k]))
		b.WriteString("\"\n")
	}
	return b.String()
}

// gitignoreWarn prints a one-line warning to stderr when path is tracked by
// git but NOT listed in .gitignore. Untracked-but-not-ignored files in a repo
// are a secret-leak risk. Silently does nothing outside a git repo or when the
// file is already ignored.
func gitignoreWarn(path string) {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	cmd := exec.Command("git", "-C", dir, "check-ignore", "--", base)
	err := cmd.Run()
	if err == nil {
		// exit 0: file IS gitignored — safe, no warning
		return
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			// exit 1: git knows this path but it is NOT ignored
			fmt.Fprintf(os.Stderr, "warning: %s is not gitignored — add it to .gitignore to avoid leaking secrets\n", path)
		case 128:
			// exit 128: not a git repo — silent
		}
	}
}
