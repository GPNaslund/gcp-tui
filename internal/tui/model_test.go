package tui

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

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
		GcloudInstalled: true, ProxyInstalled: true, HasADC: true, ADCValid: true,
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

// pressL drives the shift-L key the way bubbletea delivers it.
func pressL(m Model) (Model, tea.Cmd) {
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	return out.(Model), cmd
}

func TestLOpensPanelLoadingWithAsyncCmd(t *testing.T) {
	m, cmd := pressL(sized(testModel()))
	if !m.panel.open || !m.panel.loading {
		t.Fatalf("expected panel open+loading, got open=%v loading=%v", m.panel.open, m.panel.loading)
	}
	if m.panel.title != "logs: staging" {
		t.Fatalf("panel title: got %q", m.panel.title)
	}
	if cmd == nil {
		t.Fatal("L must return a fetch cmd so the gcloud call runs off the Update loop")
	}
}

func TestPanelDataMsgFillsViewport(t *testing.T) {
	m, _ := pressL(sized(testModel()))
	out, _ := m.Update(panelDataMsg{title: "logs: staging", content: "2026-06-20  INFO  hello"})
	m = out.(Model)
	if m.panel.loading {
		t.Fatal("panelDataMsg must clear loading")
	}
	if m.panel.err != nil {
		t.Fatalf("unexpected err: %v", m.panel.err)
	}
	if !strings.Contains(m.View(), "hello") {
		t.Fatal("viewport content not rendered in panel view")
	}
}

func TestPanelErrorRendersWithoutCrash(t *testing.T) {
	m, _ := pressL(sized(testModel()))
	out, _ := m.Update(panelDataMsg{title: "logs: staging", err: errors.New("boom")})
	m = out.(Model)
	if m.panel.loading {
		t.Fatal("panelDataMsg must clear loading even on error")
	}
	if !strings.Contains(m.View(), "boom") {
		t.Fatal("error not rendered in panel view")
	}
}

func TestEscClosesPanel(t *testing.T) {
	m, _ := pressL(sized(testModel()))
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if out.(Model).panel.open {
		t.Fatal("esc must close the panel")
	}
}

// pressD drives the shift-D key the way bubbletea delivers it.
func pressD(m Model) (Model, tea.Cmd) {
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	return out.(Model), cmd
}

func TestDOpensPanelLoadingWithAsyncCmd(t *testing.T) {
	m, cmd := pressD(sized(testModel()))
	if !m.panel.open || !m.panel.loading {
		t.Fatalf("expected panel open+loading after D, got open=%v loading=%v", m.panel.open, m.panel.loading)
	}
	if m.panel.title != "databases: staging" {
		t.Fatalf("panel title: got %q, want %q", m.panel.title, "databases: staging")
	}
	if cmd == nil {
		t.Fatal("D must return a fetch cmd so the gcloud call runs off the Update loop")
	}
}

func TestDPanelDataMsgFillsViewport(t *testing.T) {
	m, _ := pressD(sized(testModel()))
	out, _ := m.Update(panelDataMsg{title: "databases: staging", content: "velora"})
	m = out.(Model)
	if m.panel.loading {
		t.Fatal("panelDataMsg must clear loading")
	}
	if !strings.Contains(m.View(), "velora") {
		t.Fatal("databases panel content not rendered in view")
	}
}

func TestQClosesOpenPanel(t *testing.T) {
	m, _ := pressD(sized(testModel()))
	if !m.panel.open {
		t.Fatal("panel must be open after D")
	}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if out.(Model).panel.open {
		t.Fatal("q must close the panel")
	}
}

func TestOpenPanelDoesNotMoveEnvSelection(t *testing.T) {
	m, _ := pressL(sized(testModel())) // staging (idx 0) selected
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if out.(Model).envIdx != 0 {
		t.Fatal("scroll keys must not change env selection while the panel is open")
	}
	if !out.(Model).panel.open {
		t.Fatal("scroll keys must not close the panel")
	}
}

// TestTunnelExitedMsgRemovesTracking proves the cockpit's bookkeeping: when a
// tracked tunnel exits, the env is dropped from m.tunnels, live flips back to
// idle, and the toast names the env (and the error on failure).
func TestTunnelExitedMsgRemovesTracking(t *testing.T) {
	m := sized(testModel())
	m.tunnels = map[string]*exec.Cmd{"staging": {}}
	m.live["staging"] = true

	out, _ := m.Update(tunnelExitedMsg{env: "staging", err: errors.New("boom")})
	m = out.(Model)

	if _, ok := m.tunnels["staging"]; ok {
		t.Fatal("tunnelExitedMsg must remove the env from tunnels")
	}
	if m.live["staging"] {
		t.Fatal("tunnelExitedMsg must flip live back to idle")
	}
	if !strings.Contains(m.toast, "staging") || !strings.Contains(m.toast, "boom") {
		t.Fatalf("toast must surface the env and the error, got %q", m.toast)
	}
}

// TestKillAllTunnelsSIGTERMsTrackedProcess is the die-on-quit safety check: a
// real short-lived child must actually receive SIGTERM from killAllTunnels.
func TestKillAllTunnelsSIGTERMsTrackedProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start stand-in process: %v", err)
	}

	m := sized(testModel())
	m.tunnels = map[string]*exec.Cmd{"staging": cmd}
	m.killAllTunnels()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		// SIGTERM makes Wait return a non-nil exit error; a nil error would
		// mean the process exited on its own, not from our signal.
		if err == nil {
			t.Fatal("process exited cleanly; killAllTunnels did not signal it")
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("process still running 5s after killAllTunnels — no SIGTERM delivered")
	}
}

// TestQuitCleanupFilterSIGTERMsTrackedProcess proves the signal-driven quit path:
// Bubble Tea's own SIGINT/SIGTERM handling injects a QuitMsg that exits without
// touching Update/handleKey, so the WithFilter hook is the only thing that cleans
// up a tracked tunnel on `kill <pid>`. A real short-lived child tracked in the
// model must actually receive SIGTERM when the filter sees a tea.QuitMsg.
func TestQuitCleanupFilterSIGTERMsTrackedProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start stand-in process: %v", err)
	}

	m := sized(testModel())
	m.tunnels = map[string]*exec.Cmd{"staging": cmd}

	got := QuitCleanupFilter(m, tea.QuitMsg{})
	if _, ok := got.(tea.QuitMsg); !ok {
		t.Fatalf("filter must return the message unchanged, got %T", got)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("process exited cleanly; QuitCleanupFilter did not signal it")
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("process still running 5s after QuitCleanupFilter — no SIGTERM delivered")
	}
}
