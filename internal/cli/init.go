package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
	"github.com/gpnaslund/gcp-tui/internal/gcloud"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Discover Cloud SQL instances via gcloud and add them to your config",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInit()
		},
	}
}

func runInit() error {
	if _, err := doctor.Ensure(true); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	project, err := pickProject()
	if err != nil {
		return err
	}
	if project == "" {
		return fmt.Errorf("no project selected")
	}

	instances, err := gcloud.ListInstances(project)
	if err != nil {
		return fmt.Errorf("listing Cloud SQL instances in %s: %w", project, err)
	}
	if len(instances) == 0 {
		return fmt.Errorf("no Cloud SQL instances found in %s", project)
	}

	chosen, err := pickInstances(instances)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		fmt.Println("Nothing selected.")
		return nil
	}

	for _, inst := range chosen {
		env := buildEnv(cfg, project, inst)
		if err := configureEnv(&env); err != nil {
			return err
		}
		cfg.Envs = append(cfg.Envs, env)
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	path, _ := config.Path()
	fmt.Printf("\nSaved %d environment(s) to %s\n", len(chosen), path)
	return nil
}

// pickProject offers a select over the projects gcloud can enumerate, falling
// back to manual entry when the account cannot list projects.
func pickProject() (string, error) {
	projects, err := gcloud.ListProjects()
	if err != nil || len(projects) == 0 {
		var manual string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("GCP project id").
				Description("(could not list projects automatically)").
				Value(&manual),
		))
		if ferr := form.Run(); ferr != nil {
			return "", ferr
		}
		return strings.TrimSpace(manual), nil
	}

	opts := make([]huh.Option[string], 0, len(projects))
	for _, p := range projects {
		label := p.ProjectID
		if p.Name != "" {
			label = fmt.Sprintf("%s (%s)", p.ProjectID, p.Name)
		}
		opts = append(opts, huh.NewOption(label, p.ProjectID))
	}
	var chosen string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Project").Options(opts...).Value(&chosen),
	))
	if err := form.Run(); err != nil {
		return "", err
	}
	return chosen, nil
}

func pickInstances(instances []gcloud.Instance) ([]gcloud.Instance, error) {
	byConn := make(map[string]gcloud.Instance, len(instances))
	opts := make([]huh.Option[string], 0, len(instances))
	for _, i := range instances {
		byConn[i.ConnectionName] = i
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s  [%s]", i.Name, i.ConnectionName), i.ConnectionName))
	}

	var picked []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Instances to add").
			Description("Space to toggle, Enter to confirm").
			Options(opts...).
			Value(&picked),
	))
	if err := form.Run(); err != nil {
		return nil, err
	}

	out := make([]gcloud.Instance, 0, len(picked))
	for _, c := range picked {
		out = append(out, byConn[c])
	}
	return out, nil
}

func buildEnv(cfg *config.Config, project string, inst gcloud.Instance) config.Env {
	addr, port := cfg.NextSlot()
	return config.Env{
		Name:     defaultName(inst.Name),
		Project:  project,
		Instance: inst.ConnectionName,
		Address:  addr,
		Port:     port,
		Confirm:  looksLikeProd(project, inst.Name),
	}
}

// configureEnv lets the operator confirm or tweak the proposed values before
// the env is committed.
func configureEnv(e *config.Env) error {
	addr := e.Address
	port := strconv.Itoa(e.Port)
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Short name").Value(&e.Name),
		huh.NewInput().Title("Reserved loopback address").Value(&addr),
		huh.NewInput().Title("Reserved port").Value(&port),
		huh.NewConfirm().Title("Require typed confirmation before connecting? (recommended for prod)").Value(&e.Confirm),
		huh.NewConfirm().Title("Use IAM database authentication (--auto-iam-authn)?").Value(&e.IAMAuth),
	))
	if err := form.Run(); err != nil {
		return err
	}
	e.Address = strings.TrimSpace(addr)
	p, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", port, err)
	}
	e.Port = p
	return nil
}

// defaultName proposes the trailing segment of the instance name (e.g.
// "velora-staging" -> "staging").
func defaultName(instanceName string) string {
	if i := strings.LastIndex(instanceName, "-"); i >= 0 && i < len(instanceName)-1 {
		return instanceName[i+1:]
	}
	return instanceName
}

func looksLikeProd(project, instance string) bool {
	return strings.Contains(strings.ToLower(project+" "+instance), "prod")
}
