// Package tui is the interactive cockpit: a master-detail view over the same
// core (config, doctor, proxy, secret) the CLI uses. It owns the screen; tunnels
// run as tracked background proxies (proxy.StartBackground) so several can be
// live at once while the cockpit stays interactive. They are session-scoped:
// every tracked tunnel is SIGTERMed on quit (die-on-quit), so a normal exit
// never leaks a proxy.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/atotto/clipboard"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/doctor"
	"github.com/gpnaslund/gcp-tui/internal/proxy"
	"github.com/gpnaslund/gcp-tui/internal/secret"
)

type focusZone int

const (
	focusEnv focusZone = iota
	focusProfiles
	focusConfirm
)

// Model is the cockpit state.
type Model struct {
	cfg  *config.Config
	doc  doctor.Result
	live map[string]bool // env name -> slot currently has a listener

	// tunnels tracks background proxies this cockpit started, keyed by env name.
	// They are session-scoped: killAllTunnels SIGTERMs every entry on quit.
	tunnels map[string]*exec.Cmd

	w, h    int
	envIdx  int
	profIdx int
	focus   focusZone

	confirmInput string
	toast        string

	panel panel
}

type tunnelExitedMsg struct {
	env string
	err error
}
type reloadMsg struct{}

// New builds a cockpit over already-loaded config and doctor state.
func New(cfg *config.Config, doc doctor.Result) Model {
	m := Model{cfg: cfg, doc: doc, focus: focusEnv}
	m.refreshLive()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		w, h := m.panelViewportSize()
		m.panel.vp.Width, m.panel.vp.Height = w, h
		return m, nil
	case panelDataMsg:
		m.panel.loading = false
		m.panel.err = msg.err
		if msg.title != "" {
			m.panel.title = msg.title
		}
		if msg.err == nil {
			m.panel.vp.SetContent(msg.content)
			m.panel.vp.GotoTop()
		}
		return m, nil
	case tunnelExitedMsg:
		delete(m.tunnels, msg.env)
		m.refreshLive()
		if msg.err != nil {
			m.toast = "tunnel exited: " + msg.env + " (" + msg.err.Error() + ")"
		} else {
			m.toast = "tunnel exited: " + msg.env
		}
		return m, nil
	case reloadMsg:
		if cfg, err := config.Load(); err == nil {
			m.cfg = cfg
		}
		m.doc, _ = doctor.Inspect()
		m.clampIdx()
		m.refreshLive()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.toast = ""
	key := msg.String()

	if m.panel.open {
		switch key {
		case "esc", "q":
			m.panel.open = false
		case "ctrl+c":
			m.killAllTunnels()
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.panel.vp, cmd = m.panel.vp.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// focusConfirm is the interactive prod gate: the TUI equivalent of authorizeWrite.
	// The user must type the env name before startBackgroundTunnel is called. No
	// tunnel runs for confirm=true envs until this check passes.
	if m.focus == focusConfirm {
		switch key {
		case "esc":
			m.focus, m.confirmInput = focusEnv, ""
		case "enter":
			e := m.selectedEnv()
			if e != nil && m.confirmInput == e.Name {
				m.focus, m.confirmInput = focusEnv, ""
				return m.startBackgroundTunnel(*e)
			}
			m.toast, m.confirmInput = "confirmation did not match", ""
		case "backspace":
			if n := len(m.confirmInput); n > 0 {
				m.confirmInput = m.confirmInput[:n-1]
			}
		default:
			if len(key) == 1 {
				m.confirmInput += key
			}
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		m.killAllTunnels()
		return m, tea.Quit
	case "up", "k":
		if m.focus == focusProfiles {
			m.profIdx = clamp(m.profIdx-1, 0, m.profCount()-1)
		} else {
			m.envIdx, m.profIdx = clamp(m.envIdx-1, 0, len(m.cfg.Envs)-1), 0
		}
	case "down", "j":
		if m.focus == focusProfiles {
			m.profIdx = clamp(m.profIdx+1, 0, m.profCount()-1)
		} else {
			m.envIdx, m.profIdx = clamp(m.envIdx+1, 0, len(m.cfg.Envs)-1), 0
		}
	case "tab", "right", "l":
		if m.focus == focusEnv && m.profCount() > 0 {
			m.focus = focusProfiles
			m.profIdx = clamp(m.profIdx, 0, m.profCount()-1)
		}
	case "left", "h", "esc":
		m.focus = focusEnv
	case "enter":
		e := m.selectedEnv()
		if e == nil {
			return m, nil
		}
		if m.focus == focusProfiles {
			m.copyConn(e, m.selectedProfile())
			return m, nil
		}
		if e.Confirm {
			m.focus, m.confirmInput = focusConfirm, ""
			return m, nil
		}
		return m.startBackgroundTunnel(*e)
	case "c":
		e := m.selectedEnv()
		if e == nil {
			return m, nil
		}
		m.copyConn(e, m.selectedProfile())
	case "p":
		if e := m.selectedEnv(); e != nil {
			return m, execSelf("profile", "add", e.Name)
		}
	case "i":
		return m, execSelf("init")
	case "s":
		if e := m.selectedEnv(); e != nil {
			return m, execSelf("secrets", "pull", e.Name)
		}
	case "L":
		if e := m.selectedEnv(); e != nil {
			m.panel.open, m.panel.loading, m.panel.err = true, true, nil
			m.panel.title = "logs: " + e.Name
			w, h := m.panelViewportSize()
			m.panel.vp = viewport.New(w, h)
			return m, fetchLogsCmd(*e)
		}
	case "D":
		if e := m.selectedEnv(); e != nil {
			m.panel.open, m.panel.loading, m.panel.err = true, true, nil
			m.panel.title = "databases: " + e.Name
			w, h := m.panelViewportSize()
			m.panel.vp = viewport.New(w, h)
			return m, fetchDatabasesCmd(*e)
		}
	case "U":
		if e := m.selectedEnv(); e != nil {
			m.panel.open, m.panel.loading, m.panel.err = true, true, nil
			m.panel.title = "users: " + e.Name
			w, h := m.panelViewportSize()
			m.panel.vp = viewport.New(w, h)
			return m, fetchUsersCmd(*e)
		}
	case "I":
		if e := m.selectedEnv(); e != nil {
			m.panel.open, m.panel.loading, m.panel.err = true, true, nil
			m.panel.title = "instance: " + e.Name
			w, h := m.panelViewportSize()
			m.panel.vp = viewport.New(w, h)
			return m, fetchDescribeCmd(*e)
		}
	case "B":
		if e := m.selectedEnv(); e != nil {
			m.panel.open, m.panel.loading, m.panel.err = true, true, nil
			m.panel.title = "backups: " + e.Name
			w, h := m.panelViewportSize()
			m.panel.vp = viewport.New(w, h)
			return m, fetchBackupsCmd(*e)
		}
	case "x":
		if e := m.selectedEnv(); e != nil && m.live[e.Name] {
			if cmd := m.tunnels[e.Name]; cmd != nil {
				// Tracked: SIGTERM it; its waitTunnel fires tunnelExitedMsg,
				// which removes it from tracking and refreshes live state.
				_ = cmd.Process.Signal(syscall.SIGTERM)
				m.toast = "stopping tunnel: " + e.Name
				return m, nil
			}
			// Live but untracked (a proxy this cockpit did not start): hand off
			// to `down`, which locates and kills the listener on the slot.
			return m, execSelf("down", e.Name)
		}
	case "d":
		m.doc, _ = doctor.Inspect()
		m.refreshLive()
		m.toast = "refreshed"
	}
	return m, nil
}

func (m *Model) copyConn(e *config.Env, p *config.Profile) {
	if p == nil {
		m.toast = "no profile — press p to add one"
		return
	}
	pw := ""
	if !e.IAMAuth {
		got, err := secret.Get(e.Name, p.Name)
		if err != nil {
			m.toast = "no stored password for " + p.Name
			return
		}
		pw = got
	}
	if err := clipboard.WriteAll(e.ConnString(*p, pw)); err != nil {
		m.toast = "clipboard unavailable (install wl-clipboard or xclip)"
		return
	}
	m.toast = "copied " + e.Name + "/" + p.Name + " connection string"
}

// startBackgroundTunnel launches a tracked background proxy for the env and keeps
// the cockpit interactive, so several tunnels can be live at once. For confirm=true
// envs it is only reached after focusConfirm (the interactive prod gate) has
// validated the typed env name — it is never called directly on Enter for those
// envs. The returned tea.Cmd waits on the proxy and emits tunnelExitedMsg when it
// dies, which removes it from tracking.
func (m Model) startBackgroundTunnel(e config.Env) (tea.Model, tea.Cmd) {
	if m.tunnels[e.Name] != nil {
		m.toast = "tunnel already live: " + e.Name
		return m, nil
	}
	cmd, err := proxy.StartBackground(e)
	if err != nil {
		m.toast = "tunnel failed: " + err.Error()
		return m, nil
	}
	if m.tunnels == nil {
		m.tunnels = make(map[string]*exec.Cmd)
	}
	m.tunnels[e.Name] = cmd
	m.refreshLive()
	if path, perr := proxy.LogFilePath(e); perr == nil {
		m.toast = "tunnel up: " + e.Name + " (log: " + path + ")"
	} else {
		m.toast = "tunnel up: " + e.Name
	}
	return m, waitTunnel(e.Name, cmd)
}

// waitTunnel blocks on the proxy's exit and reports it back to the cockpit as a
// tunnelExitedMsg, so a tunnel dying on its own (or via SIGTERM from x/quit) is
// reflected in tracking and live state.
func waitTunnel(env string, cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg { return tunnelExitedMsg{env: env, err: cmd.Wait()} }
}

// killAllTunnels SIGTERMs every tracked background tunnel. The cockpit runs the
// terminal in raw mode, so child proxies receive no signal on a normal quit; this
// makes the tunnels session-scoped — a clean exit never leaks a proxy.
//
// It is idempotent: signalling an already-dead process returns an error, which is
// discarded, so calling it twice (e.g. from both the keystroke path and the quit
// filter) is safe.
func (m *Model) killAllTunnels() {
	for _, cmd := range m.tunnels {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}
}

// QuitCleanupFilter is a tea.WithFilter hook that kills every tracked tunnel on
// any quit, including the ones the keystroke paths never see. Bubble Tea installs
// its own SIGINT/SIGTERM handler that injects InterruptMsg/QuitMsg and exits
// WITHOUT routing through Update/handleKey, so `kill <pid>` (SIGTERM) or an OS
// SIGINT would otherwise leak the tracked proxy (prod included). The filter runs
// on every message before Bubble Tea's QuitMsg/InterruptMsg short-circuit, so it
// catches both the keystroke quits and the signal-driven ones. The tunnels map is
// a reference type, so the value-copy model still signals the live processes.
//
// It returns the message unchanged. killAllTunnels is idempotent, so the
// belt-and-suspenders killAllTunnels calls in handleKey's q/ctrl+c paths remain
// safe alongside this filter.
//
// SIGKILL and SIGHUP (terminal close) can still orphan a tunnel — Bubble Tea
// cannot intercept those. That is acceptable: an orphan shows as ● live on the
// next launch and is cleanable via x or `gcp-tui down`. We deliberately do not
// use Pdeathsig (Go delivers it per-thread, which makes it fragile).
func QuitCleanupFilter(model tea.Model, msg tea.Msg) tea.Msg {
	switch msg.(type) {
	case tea.QuitMsg, tea.InterruptMsg:
		if m, ok := model.(Model); ok {
			m.killAllTunnels()
		}
	}
	return msg
}

// execSelf runs one of this binary's own subcommands (e.g. profile add, init) in
// the released terminal, then reloads config on return.
func execSelf(args ...string) tea.Cmd {
	exe, err := os.Executable()
	if err != nil {
		exe = "gcp-tui"
	}
	return tea.ExecProcess(exec.Command(exe, args...), func(error) tea.Msg { return reloadMsg{} })
}

func (m *Model) refreshLive() {
	m.live = make(map[string]bool, len(m.cfg.Envs))
	for _, e := range m.cfg.Envs {
		m.live[e.Name] = proxy.SlotBusy(e)
	}
}

func (m *Model) clampIdx() {
	m.envIdx = clamp(m.envIdx, 0, len(m.cfg.Envs)-1)
	m.profIdx = clamp(m.profIdx, 0, m.profCount()-1)
}

func (m Model) selectedEnv() *config.Env {
	if len(m.cfg.Envs) == 0 {
		return nil
	}
	return &m.cfg.Envs[clamp(m.envIdx, 0, len(m.cfg.Envs)-1)]
}

func (m Model) profCount() int {
	if e := m.selectedEnv(); e != nil {
		return len(e.Profiles)
	}
	return 0
}

func (m Model) selectedProfile() *config.Profile {
	e := m.selectedEnv()
	if e == nil || len(e.Profiles) == 0 {
		return nil
	}
	return &e.Profiles[clamp(m.profIdx, 0, len(e.Profiles)-1)]
}

// ── view ─────────────────────────────────────────────────────────────────

// paneLayout returns the cockpit's body geometry: the total widths of the left
// and right panes and the shared body height. Sizing the panel viewport and
// View() draw from this one source so they stay in lockstep.
func (m Model) paneLayout() (leftTotal, rightTotal, bodyH int) {
	leftTotal = 34
	if m.w < 96 {
		leftTotal = m.w / 3
	}
	leftTotal = clamp(leftTotal, 22, m.w-24)
	return leftTotal, m.w - leftTotal, m.h - 4
}

// panelViewportSize is the inner content size for the panel's viewport: the
// right pane minus its border+padding (4 cols, 2 rows) and minus the panel's
// own title+rule (2 rows).
func (m Model) panelViewportSize() (w, h int) {
	_, rightTotal, bodyH := m.paneLayout()
	return clamp(rightTotal-4, 1, rightTotal), clamp(bodyH-4, 1, bodyH)
}

func (m Model) View() string {
	if m.w < 64 || m.h < 16 {
		return "gcp-tui — enlarge the terminal (min 64×16)"
	}
	leftTotal, rightTotal, bodyH := m.paneLayout()

	leftBorder, rightBorder := line, line
	if m.focus == focusEnv {
		leftBorder = m.focusColor()
	} else {
		rightBorder = m.focusColor()
	}

	rightBody := m.renderInspector(rightTotal - 4)
	if m.panel.open {
		rightBorder = m.focusColor()
		rightBody = m.panel.renderPanel(rightTotal - 4)
	}

	left := pane(m.renderEnvList(leftTotal-4), leftTotal, bodyH, leftBorder)
	right := pane(rightBody, rightTotal, bodyH, rightBorder)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(m.w), body, m.renderFooter(m.w))
}

func (m Model) focusColor() lipgloss.Color {
	if e := m.selectedEnv(); e != nil {
		return riskColor(*e)
	}
	return cool
}

func (m Model) renderHeader(width int) string {
	title := lipgloss.NewStyle().Foreground(cool).Bold(true).Render("gcp-tui")
	acct := m.doc.ActiveAccount
	if acct == "" {
		acct = "not logged in"
	}
	right := lipgloss.NewStyle().Foreground(muted).Render(acct) + "  " +
		statusPill("ADC", m.doc.HasADC && m.doc.ADCValid) + " " + statusPill("proxy", m.doc.ProxyInstalled)
	rule := lipgloss.NewStyle().Foreground(line).Render(strings.Repeat("─", width))
	return fitRow(title, right, width) + "\n" + rule
}

func (m Model) renderEnvList(width int) string {
	title := lipgloss.NewStyle().Foreground(muted).Bold(true).Render("ENVIRONMENTS")
	if len(m.cfg.Envs) == 0 {
		empty := lipgloss.NewStyle().Foreground(muted).Render("none yet — press i to discover")
		return title + "\n\n" + empty
	}
	rows := make([]string, 0, len(m.cfg.Envs))
	for i, e := range m.cfg.Envs {
		risk := riskColor(e)
		gutter := "  "
		nameStyle := lipgloss.NewStyle().Foreground(ink)
		if i == m.envIdx {
			gutter = lipgloss.NewStyle().Foreground(risk).Render("▎ ")
			nameStyle = lipgloss.NewStyle().Foreground(risk).Bold(true)
		}
		var pill string
		if m.live[e.Name] {
			pill = lipgloss.NewStyle().Foreground(risk).Render("● live")
		} else {
			pill = lipgloss.NewStyle().Foreground(muted).Render("○ idle")
		}
		rows = append(rows, fitRow(gutter+nameStyle.Render(e.Name), pill, width))
	}
	return title + "\n\n" + strings.Join(rows, "\n")
}

func (m Model) renderInspector(width int) string {
	e := m.selectedEnv()
	if e == nil {
		return lipgloss.NewStyle().Foreground(muted).Render("No environments.\nPress i to discover, or run `gcp-tui init`.")
	}
	risk := riskColor(*e)

	head := fitRow(lipgloss.NewStyle().Foreground(risk).Bold(true).Render(e.Name), riskTag(*e), width)
	rule := lipgloss.NewStyle().Foreground(line).Render(strings.Repeat("─", width))

	slotVal := lipgloss.NewStyle().Foreground(risk).Bold(true).Render("▸ " + e.Address + ":" + strconv.Itoa(e.Port))
	livePill := lipgloss.NewStyle().Foreground(muted).Render("○ idle")
	if m.live[e.Name] {
		livePill = lipgloss.NewStyle().Foreground(risk).Bold(true).Render("◍ LIVE")
	}
	slotLine := fitRow("  "+label("slot")+"  "+slotVal, livePill, width)
	instLine := "  " + label("inst") + "  " + lipgloss.NewStyle().Foreground(muted).Render(truncate(e.Instance, width-10))
	auth := "password"
	if e.IAMAuth {
		auth = "IAM"
	}
	authLine := "  " + label("auth") + "  " + lipgloss.NewStyle().Foreground(ink).Render(fmt.Sprintf("%s · %d profile(s)", auth, len(e.Profiles)))

	lines := []string{head, rule, "", slotLine, instLine, authLine, "", "  " + label("profiles")}
	if len(e.Profiles) == 0 {
		lines = append(lines, "    "+lipgloss.NewStyle().Foreground(muted).Render("none — press p to add"))
	}
	for i, p := range e.Profiles {
		marker := "  "
		ps := lipgloss.NewStyle().Foreground(ink)
		if m.focus == focusProfiles && i == m.profIdx {
			marker = lipgloss.NewStyle().Foreground(risk).Render("› ")
			ps = lipgloss.NewStyle().Foreground(risk).Bold(true)
		}
		lines = append(lines, "    "+marker+ps.Render(p.Name)+"  "+lipgloss.NewStyle().Foreground(muted).Render(p.User))
	}

	if m.focus == focusConfirm {
		warn := lipgloss.NewStyle().Foreground(hot).Bold(true).Render("⚠ OPEN PRODUCTION TUNNEL")
		prompt := lipgloss.NewStyle().Foreground(hot).Render(fmt.Sprintf("type %q to confirm: ", e.Name)) +
			lipgloss.NewStyle().Foreground(ink).Render(m.confirmInput+"▌")
		lines = append(lines, "", warn, prompt)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFooter(width int) string {
	var help string
	switch {
	case m.panel.open:
		help = "↑↓ scroll · esc/q close · ctrl+c quit"
	case m.focus == focusProfiles:
		help = "↑↓ profile · c/⏎ copy conn · ← back · q quit"
	case m.focus == focusConfirm:
		help = "type the env name · ⏎ confirm · esc cancel"
	default:
		help = "↑↓ move · ⏎ tunnel · L logs · D dbs · U users · I info · B backups\n" +
			lipgloss.NewStyle().Foreground(muted).Render("x down · c copy · p profile · i discover · s pull · d doctor · q quit")
	}
	toast := " "
	if m.toast != "" {
		toast = lipgloss.NewStyle().Foreground(amber).Render(m.toast)
	}
	return toast + "\n" + lipgloss.NewStyle().Foreground(muted).Render(help)
}
