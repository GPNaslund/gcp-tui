package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
	"github.com/gpnaslund/gcp-tui/internal/gcloud"
	"github.com/gpnaslund/gcp-tui/internal/secret"
	"github.com/gpnaslund/gcp-tui/internal/secretmanager"
)

// registerTools wires every tool onto the server. Read tools are marked
// read-only; the two writes are additive (a backup / a new secret version), not
// destructive. AddTool infers each tool's input and output JSON schema from the
// handler's argument and result types.
func registerTools(s *mcp.Server) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	additive := &mcp.ToolAnnotations{DestructiveHint: boolPtr(false)}

	mcp.AddTool(s, &mcp.Tool{Name: "status", Annotations: readOnly,
		Description: "Report gcloud account, ADC validity, installed tools, and the configured environments."}, handleStatus)
	mcp.AddTool(s, &mcp.Tool{Name: "list_environments", Annotations: readOnly,
		Description: "List the configured environments, their reserved loopback slot, profiles, and whether each is protected (confirm=true)."}, handleListEnvironments)
	mcp.AddTool(s, &mcp.Tool{Name: "sql_databases", Annotations: readOnly,
		Description: "List the databases in an environment's Cloud SQL instance."}, handleSQLDatabases)
	mcp.AddTool(s, &mcp.Tool{Name: "sql_users", Annotations: readOnly,
		Description: "List the users in an environment's Cloud SQL instance."}, handleSQLUsers)
	mcp.AddTool(s, &mcp.Tool{Name: "sql_describe", Annotations: readOnly,
		Description: "Describe an environment's Cloud SQL instance (version, region, state, tier, disk, backup config, IPs)."}, handleSQLDescribe)
	mcp.AddTool(s, &mcp.Tool{Name: "logs", Annotations: readOnly,
		Description: "Read recent Cloud Logging entries for an environment's Cloud SQL instance."}, handleLogs)
	mcp.AddTool(s, &mcp.Tool{Name: "backups_list", Annotations: readOnly,
		Description: "List automated and on-demand backups for an environment's Cloud SQL instance."}, handleBackupsList)
	mcp.AddTool(s, &mcp.Tool{Name: "secrets_list", Annotations: readOnly,
		Description: "List Secret Manager secret names in an environment's project. Values are never returned."}, handleSecretsList)
	mcp.AddTool(s, &mcp.Tool{Name: "connection_string", Annotations: readOnly,
		Description: "Build a Postgres connection string for an environment profile. The password is omitted unless include_password=true."}, handleConnectionString)

	mcp.AddTool(s, &mcp.Tool{Name: "backups_create", Annotations: additive,
		Description: "Take an on-demand backup of an environment's Cloud SQL instance. Gated: protected environments are refused; non-prod requires authorize=true."}, handleBackupsCreate)
	mcp.AddTool(s, &mcp.Tool{Name: "secrets_set", Annotations: additive,
		Description: "Add a new version of a Secret Manager secret in an environment's project. Gated: protected environments are refused; non-prod requires authorize=true. The value passes through the agent."}, handleSecretsSet)
}

func boolPtr(b bool) *bool { return &b }

// envInput is the shared single-argument shape: an environment name.
type envInput struct {
	Env string `json:"env" jsonschema:"the configured environment name (see list_environments)"`
}

// emptyInput is the argument type for tools that take no parameters.
type emptyInput struct{}

// envInfo is the agent-facing view of a configured environment.
type envInfo struct {
	Name     string   `json:"name"`
	Project  string   `json:"project"`
	Instance string   `json:"instance"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Confirm  bool     `json:"confirm"`
	IAMAuth  bool     `json:"iam_auth"`
	Profiles []string `json:"profiles"`
}

func envInfos(envs []config.Env) []envInfo {
	out := make([]envInfo, len(envs))
	for i, e := range envs {
		profiles := make([]string, len(e.Profiles))
		for j, p := range e.Profiles {
			profiles[j] = p.Name
		}
		out[i] = envInfo{
			Name:     e.Name,
			Project:  e.Project,
			Instance: e.Instance,
			Address:  e.Address,
			Port:     e.Port,
			Confirm:  e.Confirm,
			IAMAuth:  e.IAMAuth,
			Profiles: profiles,
		}
	}
	return out
}

type statusOutput struct {
	Account      string    `json:"account"`
	HasAccount   bool      `json:"has_account"`
	ADCPresent   bool      `json:"adc_present"`
	ADCValid     bool      `json:"adc_valid"`
	Gcloud       bool      `json:"gcloud_installed"`
	Proxy        bool      `json:"proxy_installed"`
	Psql         bool      `json:"psql_installed"`
	Environments []envInfo `json:"environments"`
}

func handleStatus(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, statusOutput, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, statusOutput{}, err
	}
	doc, err := doctor.Inspect()
	if err != nil {
		return nil, statusOutput{}, err
	}
	return nil, statusOutput{
		Account:      doc.ActiveAccount,
		HasAccount:   doc.HasAccount,
		ADCPresent:   doc.HasADC,
		ADCValid:     doc.ADCValid,
		Gcloud:       doc.GcloudInstalled,
		Proxy:        doc.ProxyInstalled,
		Psql:         doc.PsqlInstalled,
		Environments: envInfos(cfg.Envs),
	}, nil
}

type environmentsOutput struct {
	Environments []envInfo `json:"environments"`
}

func handleListEnvironments(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, environmentsOutput, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, environmentsOutput{}, err
	}
	return nil, environmentsOutput{Environments: envInfos(cfg.Envs)}, nil
}

type databasesOutput struct {
	Databases []gcloud.Database `json:"databases"`
}

func handleSQLDatabases(_ context.Context, _ *mcp.CallToolRequest, in envInput) (*mcp.CallToolResult, databasesOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, databasesOutput{}, err
	}
	dbs, err := gcloud.ListDatabases(env.Project, env.InstanceName())
	if err != nil {
		return nil, databasesOutput{}, err
	}
	return nil, databasesOutput{Databases: dbs}, nil
}

type usersOutput struct {
	Users []gcloud.SQLUser `json:"users"`
}

func handleSQLUsers(_ context.Context, _ *mcp.CallToolRequest, in envInput) (*mcp.CallToolResult, usersOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, usersOutput{}, err
	}
	users, err := gcloud.ListUsers(env.Project, env.InstanceName())
	if err != nil {
		return nil, usersOutput{}, err
	}
	return nil, usersOutput{Users: users}, nil
}

func handleSQLDescribe(_ context.Context, _ *mcp.CallToolRequest, in envInput) (*mcp.CallToolResult, gcloud.InstanceDetail, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, gcloud.InstanceDetail{}, err
	}
	detail, err := gcloud.DescribeInstance(env.Project, env.InstanceName())
	if err != nil {
		return nil, gcloud.InstanceDetail{}, err
	}
	return nil, detail, nil
}

type logsInput struct {
	Env      string `json:"env" jsonschema:"the configured environment name"`
	Since    string `json:"since,omitempty" jsonschema:"gcloud freshness window, e.g. 1h, 30m, 2d (default 1h)"`
	Severity string `json:"severity,omitempty" jsonschema:"minimum severity, e.g. ERROR, WARNING"`
	Grep     string `json:"grep,omitempty" jsonschema:"only entries containing this substring"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum entries to return (default 50)"`
}

type logsOutput struct {
	Entries []gcloud.LogEntry `json:"entries"`
}

func handleLogs(_ context.Context, _ *mcp.CallToolRequest, in logsInput) (*mcp.CallToolResult, logsOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, logsOutput{}, err
	}
	entries, err := gcloud.ReadLogs(gcloud.LogQuery{
		Project:    env.Project,
		DatabaseID: env.DatabaseID(),
		Freshness:  in.Since,
		Severity:   in.Severity,
		Grep:       in.Grep,
		Limit:      in.Limit,
	})
	if err != nil {
		return nil, logsOutput{}, err
	}
	return nil, logsOutput{Entries: entries}, nil
}

type backupsListOutput struct {
	Backups []gcloud.Backup `json:"backups"`
}

func handleBackupsList(_ context.Context, _ *mcp.CallToolRequest, in envInput) (*mcp.CallToolResult, backupsListOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, backupsListOutput{}, err
	}
	backups, err := gcloud.ListBackups(env.Project, env.InstanceName())
	if err != nil {
		return nil, backupsListOutput{}, err
	}
	return nil, backupsListOutput{Backups: backups}, nil
}

type secretsOutput struct {
	Secrets []string `json:"secrets"`
}

func handleSecretsList(_ context.Context, _ *mcp.CallToolRequest, in envInput) (*mcp.CallToolResult, secretsOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, secretsOutput{}, err
	}
	secrets, err := secretmanager.List(env.Project)
	if err != nil {
		return nil, secretsOutput{}, err
	}
	names := make([]string, len(secrets))
	for i, s := range secrets {
		names[i] = s.Name
	}
	return nil, secretsOutput{Secrets: names}, nil
}

type connInput struct {
	Env             string `json:"env" jsonschema:"the configured environment name"`
	Profile         string `json:"profile,omitempty" jsonschema:"profile name; optional when the environment has exactly one"`
	IncludePassword bool   `json:"include_password,omitempty" jsonschema:"embed the stored password in the URL; default false so the password is not exposed to the agent. Ignored for IAM-auth environments, which have no stored password"`
}

type connOutput struct {
	ConnectionString string `json:"connection_string"`
	HasPassword      bool   `json:"has_password"`
}

func handleConnectionString(_ context.Context, _ *mcp.CallToolRequest, in connInput) (*mcp.CallToolResult, connOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, connOutput{}, err
	}
	prof, err := resolveProfile(env, in.Profile)
	if err != nil {
		return nil, connOutput{}, err
	}
	password := ""
	if in.IncludePassword && !env.IAMAuth {
		pw, gerr := secret.Get(env.Name, prof.Name)
		if gerr != nil {
			return nil, connOutput{}, fmt.Errorf("no stored password for %s/%s: %w", env.Name, prof.Name, gerr)
		}
		password = pw
	}
	return nil, connOutput{
		ConnectionString: env.ConnString(*prof, password),
		HasPassword:      password != "",
	}, nil
}

type backupsCreateInput struct {
	Env       string `json:"env" jsonschema:"the configured environment name"`
	Authorize bool   `json:"authorize,omitempty" jsonschema:"must be true to perform this non-prod write; protected environments are refused regardless"`
}

type backupsCreateOutput struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

func handleBackupsCreate(_ context.Context, _ *mcp.CallToolRequest, in backupsCreateInput) (*mcp.CallToolResult, backupsCreateOutput, error) {
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, backupsCreateOutput{}, err
	}
	if err := authorize(*env, in.Authorize); err != nil {
		return nil, backupsCreateOutput{}, err
	}
	out, err := gcloud.CreateBackupCaptured(env.Project, env.InstanceName())
	if err != nil {
		return nil, backupsCreateOutput{}, err
	}
	return nil, backupsCreateOutput{Status: "backup started", Output: string(out)}, nil
}

type secretsSetInput struct {
	Env             string `json:"env" jsonschema:"the configured environment name"`
	Name            string `json:"name" jsonschema:"the secret's short name"`
	Value           string `json:"value" jsonschema:"the secret value; it travels through the agent, so do not use it for values the agent should not see"`
	CreateIfMissing bool   `json:"create_if_missing,omitempty" jsonschema:"create the secret if it does not exist yet (default false)"`
	Authorize       bool   `json:"authorize,omitempty" jsonschema:"must be true to perform this non-prod write; protected environments are refused regardless"`
}

type secretsSetOutput struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func handleSecretsSet(_ context.Context, _ *mcp.CallToolRequest, in secretsSetInput) (*mcp.CallToolResult, secretsSetOutput, error) {
	if in.Name == "" {
		return nil, secretsSetOutput{}, fmt.Errorf("name is required")
	}
	if in.Value == "" {
		return nil, secretsSetOutput{}, fmt.Errorf("value is required (empty values are refused)")
	}
	env, err := findEnv(in.Env)
	if err != nil {
		return nil, secretsSetOutput{}, err
	}
	if err := authorize(*env, in.Authorize); err != nil {
		return nil, secretsSetOutput{}, err
	}
	exists, err := secretmanager.Exists(env.Project, in.Name)
	if err != nil {
		return nil, secretsSetOutput{}, err
	}
	if !exists {
		if !in.CreateIfMissing {
			return nil, secretsSetOutput{}, fmt.Errorf("secret %q does not exist in project %s; pass create_if_missing=true to create it", in.Name, env.Project)
		}
		if err := secretmanager.Create(env.Project, in.Name); err != nil {
			return nil, secretsSetOutput{}, err
		}
	}
	version, err := secretmanager.AddVersion(env.Project, in.Name, []byte(in.Value))
	if err != nil {
		return nil, secretsSetOutput{}, err
	}
	return nil, secretsSetOutput{Status: "version added", Version: version}, nil
}
