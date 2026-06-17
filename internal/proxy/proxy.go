// Package proxy launches the Cloud SQL Auth Proxy bound to an environment's
// reserved loopback slot, under a colored banner, for the lifetime of the
// session.
package proxy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/gpnaslund/gcp-tui/internal/config"
	"github.com/gpnaslund/gcp-tui/internal/run"
)

// SlotBusy reports whether something already listens on the env's reserved
// address:port. A live listener means a stale proxy or a foreign process
// squatting the slot; either way we refuse to start a second tunnel there.
func SlotBusy(e config.Env) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", e.Address, e.Port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ListenerPIDs returns the PIDs of processes listening on the env's reserved
// address:port, using lsof. An empty slice means the slot is free. Returns an
// error only if lsof is absent from PATH; a non-zero lsof exit (no matches) is
// treated as the free case, not an error.
func ListenerPIDs(e config.Env) ([]int, error) {
	if _, err := exec.LookPath("lsof"); err != nil {
		return nil, fmt.Errorf("lsof not found on PATH; cannot locate the listener on %s:%d", e.Address, e.Port)
	}
	out, _ := exec.Command(
		"lsof", "-nP",
		fmt.Sprintf("-iTCP@%s:%d", e.Address, e.Port),
		"-sTCP:LISTEN", "-t",
	).Output()
	return parsePIDs(string(out)), nil
}

// parsePIDs parses the output of `lsof -t` (bare PIDs, one per line) into
// []int. Blank lines and non-numeric lines are silently skipped. This is the
// pure, unit-testable core of ListenerPIDs.
func parsePIDs(out string) []int {
	var pids []int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, n)
	}
	return pids
}

func banner(e config.Env) string {
	color := lipgloss.Color("10") // green
	tag := "STAGING / NON-PROD"
	if e.Confirm {
		color = lipgloss.Color("9") // red
		tag = "PRODUCTION"
	}
	style := lipgloss.NewStyle().
		Background(color).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Padding(0, 1)
	return style.Render(fmt.Sprintf("%s  %s  ->  %s:%d", tag, e.Instance, e.Address, e.Port))
}

// Command builds the cloud-sql-proxy invocation for an environment, wired to the
// current terminal. Callers run it directly (Start) or hand it to the TUI via
// tea.ExecProcess.
func Command(e config.Env) *exec.Cmd {
	args := []string{"--address", e.Address, "--port", strconv.Itoa(e.Port)}
	if e.IAMAuth {
		args = append(args, "--auto-iam-authn")
	}
	args = append(args, e.Instance)
	cmd := exec.Command("cloud-sql-proxy", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Start launches cloud-sql-proxy bound to the env's reserved slot, streaming its
// output under a banner, and blocks until interrupted (Ctrl-C), which tears the
// tunnel down cleanly.
func Start(e config.Env) error {
	if SlotBusy(e) {
		return fmt.Errorf("refusing to start: %s:%d already has a listener (stale proxy or another process); free it first", e.Address, e.Port)
	}

	cmd := Command(e)
	fmt.Println(banner(e))
	run.Echo("cloud-sql-proxy", cmd.Args[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-sig:
		fmt.Println("\nshutting down tunnel...")
		_ = cmd.Process.Signal(syscall.SIGTERM)
		<-done
		return nil
	case err := <-done:
		return err
	}
}
