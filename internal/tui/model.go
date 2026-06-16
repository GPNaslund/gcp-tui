// Package tui is the interactive cockpit: a master-detail view over the same
// core (config, doctor, proxy, secret) the CLI uses. It owns the screen; tunnels
// run via tea.ExecProcess, which releases the terminal to the proxy and resumes
// the cockpit when it exits.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

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

	w, h    int
	envIdx  int
	profIdx int
	focus   focusZone

	confirmInput string
	toast        string
}

type tunnelClosedMsg struct{ err error }
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
		return m, nil
	case tunnelClosedMsg:
		m.refreshLive()
		if msg.err != nil {
			m.toast = "tunnel exited: " + msg.err.Error()
		} else {
			m.toast = "tunnel closed"
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

	if m.focus == focusConfirm {
		switch key {
		case "esc":
			m.focus, m.confirmInput = focusEnv, ""
		case "enter":
			e := m.selectedEnv()
			if e != nil && m.confirmInput == e.Name {
				m.focus, m.confirmInput = focusEnv, ""
				return m, startTunnel(*e)
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
		return m, startTunnel(*e)
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

// startTunnel runs the proxy via ExecProcess, releasing the terminal to it and
// resuming the cockpit when it exits.
func startTunnel(e config.Env) tea.Cmd {
	if proxy.SlotBusy(e) {
		return func() tea.Msg { return tunnelClosedMsg{fmt.Errorf("%s:%d already in use", e.Address, e.Port)} }
	}
	return tea.ExecProcess(proxy.Command(e), func(err error) tea.Msg { return tunnelClosedMsg{err} })
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

func (m Model) View() string {
	if m.w < 64 || m.h < 16 {
		return "gcp-tui — enlarge the terminal (min 64×16)"
	}
	leftTotal := 34
	if m.w < 96 {
		leftTotal = m.w / 3
	}
	leftTotal = clamp(leftTotal, 22, m.w-24)
	rightTotal := m.w - leftTotal
	bodyH := m.h - 4

	leftBorder, rightBorder := line, line
	if m.focus == focusEnv {
		leftBorder = m.focusColor()
	} else {
		rightBorder = m.focusColor()
	}

	left := pane(m.renderEnvList(leftTotal-4), leftTotal, bodyH, leftBorder)
	right := pane(m.renderInspector(rightTotal-4), rightTotal, bodyH, rightBorder)
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
		statusPill("ADC", m.doc.HasADC) + " " + statusPill("proxy", m.doc.ProxyInstalled)
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
	switch m.focus {
	case focusProfiles:
		help = "↑↓ profile · c/⏎ copy conn · ← back · q quit"
	case focusConfirm:
		help = "type the env name · ⏎ confirm · esc cancel"
	default:
		help = "↑↓ move · ⏎ tunnel · c copy · p profile · i discover · s pull · d doctor · q quit"
	}
	toast := " "
	if m.toast != "" {
		toast = lipgloss.NewStyle().Foreground(amber).Render(m.toast)
	}
	return toast + "\n" + lipgloss.NewStyle().Foreground(muted).Render(help)
}
