// Package run executes external commands and echoes each one before it runs.
// Transparency is a design principle: nothing this tool does to your gcloud or
// Cloud SQL state should be hidden, because it can reach production.
package run

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var cmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

// Echo prints the exact command that is about to run, to stderr, dimmed.
func Echo(name string, args ...string) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Fprintln(os.Stderr, cmdStyle.Render("$ "+line))
}

// Output runs a command, echoing it first, and returns its stdout. Stderr is
// passed through so failures are visible.
func Output(name string, args ...string) ([]byte, error) {
	Echo(name, args...)
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// Inherit runs a command wired to the current terminal, for interactive flows
// such as `gcloud auth login`. It blocks until the command exits.
func Inherit(name string, args ...string) error {
	Echo(name, args...)
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
