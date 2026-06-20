package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Env is one declared Cloud SQL environment: a tunnel target plus the reserved
// loopback slot it always binds to. The (Address, Port) pair is the safety
// mechanism — it makes a connection string structurally encode which
// environment it targets, so prod can never be mistaken for localhost.
type Env struct {
	Name     string `toml:"name"`
	Project  string `toml:"project"`
	Instance string `toml:"instance"` // full connection name: project:region:instance
	Address  string `toml:"address"`  // reserved loopback IP, e.g. 127.0.0.2
	Port     int    `toml:"port"`     // reserved port, e.g. 15433
	IAMAuth  bool   `toml:"iam_auth"` // pass --auto-iam-authn to the proxy
	Confirm  bool   `toml:"confirm"`  // require typed confirmation before connecting

	Profiles []Profile `toml:"profile"`
}

// Profile is a stored set of database connection details attached to an Env, so
// a ready-to-use connection string can be produced without looking values up.
// The password is never stored here — it lives in the OS keyring.
type Profile struct {
	Name    string `toml:"name"`
	User    string `toml:"user"`
	DBName  string `toml:"dbname"`
	SSLMode string `toml:"sslmode"` // "disable" for the proxy's plaintext loopback hop
}

// Config is the full declared set of environments. It is the single source of
// truth; the discovery flow (init) only ever proposes entries into it.
type Config struct {
	Envs []Env `toml:"env"`
}

const (
	slotBaseOctet = 2     // 127.0.0.2 — leaves 127.0.0.1 for local Postgres
	slotBasePort  = 15433 // distinct from 5432 so the port is a second signal
)

func dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gcp-tui"), nil
}

// Path returns the absolute path of the config file.
func Path() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.toml"), nil
}

// Load reads the config, returning an empty Config (not an error) when the file
// does not exist yet — a fresh install has nothing declared.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config with owner-only permissions.
func (c *Config) Save() error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "config.toml"), data, 0o600)
}

// Find returns the environment with the given name.
func (c *Config) Find(name string) (*Env, bool) {
	for i := range c.Envs {
		if c.Envs[i].Name == name {
			return &c.Envs[i], true
		}
	}
	return nil, false
}

// NextSlot returns the next reserved (address, port) not already claimed by an
// existing environment. The first env gets 127.0.0.2:15433, the next
// 127.0.0.3:15434, and so on, in lockstep.
func (c *Config) NextSlot() (string, int) {
	used := map[string]bool{}
	for _, e := range c.Envs {
		used[fmt.Sprintf("%s:%d", e.Address, e.Port)] = true
	}
	for i := 0; i < 250; i++ {
		addr := fmt.Sprintf("127.0.0.%d", slotBaseOctet+i)
		port := slotBasePort + i
		if !used[fmt.Sprintf("%s:%d", addr, port)] {
			return addr, port
		}
	}
	return "", 0
}

// FindProfile returns the named profile on this environment.
func (e *Env) FindProfile(name string) (*Profile, bool) {
	for i := range e.Profiles {
		if e.Profiles[i].Name == name {
			return &e.Profiles[i], true
		}
	}
	return nil, false
}

// RemoveProfile drops the named profile, reporting whether it existed.
func (e *Env) RemoveProfile(name string) bool {
	for i := range e.Profiles {
		if e.Profiles[i].Name == name {
			e.Profiles = append(e.Profiles[:i], e.Profiles[i+1:]...)
			return true
		}
	}
	return false
}

// InstanceName returns the bare Cloud SQL instance name: the segment after the
// last ':' in the full connection name (project:region:instance). When the
// value has no ':', the whole string is the instance name.
func (e Env) InstanceName() string {
	if i := strings.LastIndex(e.Instance, ":"); i >= 0 {
		return e.Instance[i+1:]
	}
	return e.Instance
}

// DatabaseID is the Cloud Logging resource.labels.database_id for this env:
// "project:instance". It pairs Project with the bare instance name, matching the
// identifier Cloud Logging records against Cloud SQL log entries.
func (e Env) DatabaseID() string {
	return e.Project + ":" + e.InstanceName()
}

// ConnString builds a Postgres URL for a profile against this environment's
// reserved slot. Pass an empty password for IAM auth (the proxy injects the
// token). net/url handles escaping of special characters in user and password.
func (e Env) ConnString(p Profile, password string) string {
	sslmode := p.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	u := url.URL{
		Scheme: "postgresql",
		Host:   fmt.Sprintf("%s:%d", e.Address, e.Port),
		Path:   "/" + p.DBName,
	}
	switch {
	case password != "":
		u.User = url.UserPassword(p.User, password)
	case p.User != "":
		u.User = url.User(p.User)
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}
