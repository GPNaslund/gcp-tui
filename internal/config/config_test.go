package config

import (
	"net/url"
	"testing"
)

func TestConnString(t *testing.T) {
	e := Env{Address: "127.0.0.2", Port: 15433}
	p := Profile{User: "app", DBName: "velora"}
	got := e.ConnString(p, "s3cr3t")
	want := "postgresql://app:s3cr3t@127.0.0.2:15433/velora?sslmode=disable"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestConnStringEscapesPassword(t *testing.T) {
	e := Env{Address: "127.0.0.3", Port: 15434}
	p := Profile{User: "app", DBName: "db"}
	raw := "p@ss/w:rd?x&y"
	u, err := url.Parse(e.ConnString(p, raw))
	if err != nil {
		t.Fatalf("conn string did not parse: %v", err)
	}
	if pw, _ := u.User.Password(); pw != raw {
		t.Fatalf("password round-trip: got %q want %q", pw, raw)
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Fatalf("sslmode missing from %q", u.String())
	}
}

func TestConnStringIAMNoPassword(t *testing.T) {
	e := Env{Address: "127.0.0.2", Port: 15433}
	p := Profile{User: "gustav.naslund@velora.se", DBName: "db"}
	u, err := url.Parse(e.ConnString(p, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, hasPw := u.User.Password(); hasPw {
		t.Fatal("expected no password for IAM connection string")
	}
	if u.User.Username() != "gustav.naslund@velora.se" {
		t.Fatalf("username round-trip: got %q", u.User.Username())
	}
}

func TestInstanceNameAndDatabaseID(t *testing.T) {
	e := Env{Project: "fluted-anthem-413815", Instance: "fluted-anthem-413815:europe-north2:velora-staging"}
	if got := e.InstanceName(); got != "velora-staging" {
		t.Fatalf("InstanceName: got %q want %q", got, "velora-staging")
	}
	if got := e.DatabaseID(); got != "fluted-anthem-413815:velora-staging" {
		t.Fatalf("DatabaseID: got %q want %q", got, "fluted-anthem-413815:velora-staging")
	}
}

func TestInstanceNameColonless(t *testing.T) {
	e := Env{Project: "p", Instance: "velora-staging"}
	if got := e.InstanceName(); got != "velora-staging" {
		t.Fatalf("InstanceName colonless: got %q want %q", got, "velora-staging")
	}
	if got := e.DatabaseID(); got != "p:velora-staging" {
		t.Fatalf("DatabaseID colonless: got %q want %q", got, "p:velora-staging")
	}
}

func TestNextSlotSkipsUsed(t *testing.T) {
	c := &Config{Envs: []Env{
		{Name: "staging", Address: "127.0.0.2", Port: 15433},
	}}
	addr, port := c.NextSlot()
	if addr != "127.0.0.3" || port != 15434 {
		t.Fatalf("got %s:%d, want 127.0.0.3:15434", addr, port)
	}
}

func TestNextSlotEmpty(t *testing.T) {
	c := &Config{}
	addr, port := c.NextSlot()
	if addr != "127.0.0.2" || port != 15433 {
		t.Fatalf("got %s:%d, want 127.0.0.2:15433", addr, port)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	in := &Config{Envs: []Env{
		{Name: "staging", Project: "p", Instance: "p:r:i", Address: "127.0.0.2", Port: 15433, Confirm: false},
		{Name: "prod", Project: "q", Instance: "q:r:j", Address: "127.0.0.3", Port: 15434, Confirm: true},
	}}
	if err := in.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Envs) != 2 {
		t.Fatalf("want 2 envs, got %d", len(got.Envs))
	}
	prod, ok := got.Find("prod")
	if !ok {
		t.Fatal("prod env not found after round-trip")
	}
	if !prod.Confirm || prod.Instance != "q:r:j" {
		t.Fatalf("round-trip mismatch: %+v", prod)
	}
}

func TestMultipleProfilesPerEnvRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	in := &Config{Envs: []Env{{
		Name: "staging", Project: "p", Instance: "p:r:i", Address: "127.0.0.2", Port: 15433,
		Profiles: []Profile{
			{Name: "app", User: "app_user", DBName: "velora", SSLMode: "disable"},
			{Name: "ro", User: "readonly", DBName: "velora", SSLMode: "disable"},
		},
	}}}
	if err := in.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	env, ok := got.Find("staging")
	if !ok {
		t.Fatal("staging env missing after round-trip")
	}
	if len(env.Profiles) != 2 {
		t.Fatalf("want 2 profiles, got %d", len(env.Profiles))
	}
	ro, ok := env.FindProfile("ro")
	if !ok || ro.User != "readonly" {
		t.Fatalf("readonly profile not preserved: %+v", ro)
	}
	app, ok := env.FindProfile("app")
	if !ok || app.User != "app_user" {
		t.Fatalf("app profile not preserved: %+v", app)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Envs) != 0 {
		t.Fatalf("want empty config, got %+v", cfg)
	}
}
