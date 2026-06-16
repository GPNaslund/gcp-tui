package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
)

func testModel() Model {
	cfg := &config.Config{Envs: []config.Env{
		{
			Name: "staging", Address: "127.0.0.2", Port: 15433,
			Instance: "fluted-anthem-413815:europe-north2:velora-staging",
			Profiles: []config.Profile{{Name: "app", User: "app_user", DBName: "velora"}},
		},
		{
			Name: "prod", Address: "127.0.0.3", Port: 15434, Confirm: true,
			Instance: "velora-data:europe-north2:velora-production",
		},
	}}
	return New(cfg, doctor.Result{
		GcloudInstalled: true, ProxyInstalled: true, HasADC: true,
		HasAccount: true, ActiveAccount: "me@velora.se",
	})
}

func sized(m Model) Model {
	out, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return out.(Model)
}

func TestViewRendersKeyContent(t *testing.T) {
	out := sized(testModel()).View()
	for _, want := range []string{"gcp-tui", "ENVIRONMENTS", "staging", "127.0.0.2:15433", "app"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view missing %q", want)
		}
	}
}

func TestProdEnterAsksConfirmation(t *testing.T) {
	m := sized(testModel())
	m.envIdx = 1 // prod

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).focus != focusConfirm {
		t.Fatalf("expected confirm focus for prod, got %v", next.(Model).focus)
	}
	if cmd != nil {
		t.Fatal("prod must not start a tunnel before confirmation")
	}
}

func TestTabFocusesProfilesWhenPresent(t *testing.T) {
	m := sized(testModel()) // staging selected, has a profile
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if next.(Model).focus != focusProfiles {
		t.Fatalf("expected profiles focus, got %v", next.(Model).focus)
	}
}
