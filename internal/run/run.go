// Package run executes external commands and echoes each one before it runs.
// Transparency is a design principle: nothing this tool does to your gcloud or
// Cloud SQL state should be hidden, because it can reach production.
package run

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var cmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

// DryRun, when true, makes Output/OutputInput/Inherit print the plain command
// and return ErrDryRun without executing anything.
var DryRun bool

// ErrDryRun is returned by Output/OutputInput/Inherit when DryRun is true.
var ErrDryRun = errors.New("dry run: command not executed")

// dryRunOut is where dry-run command lines are printed. Tests replace it.
var dryRunOut io.Writer = os.Stdout

// printDryRun prints "name args..." to dryRunOut with no ANSI styling.
func printDryRun(name string, args ...string) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Fprintln(dryRunOut, line)
}

// Echo prints the exact command that is about to run, to stderr, dimmed.
func Echo(name string, args ...string) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Fprintln(os.Stderr, cmdStyle.Render("$ "+line))
}

// Output runs a command, echoing it first, and returns its stdout. Stderr is
// passed through so failures are visible.
func Output(name string, args ...string) ([]byte, error) {
	if DryRun {
		printDryRun(name, args...)
		return nil, ErrDryRun
	}
	Echo(name, args...)
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// OutputInput runs a command with input piped to its stdin, echoing only the
// command — never the input. Use this for secret payloads so the value is never
// visible in the process list or the echoed command line.
func OutputInput(input []byte, name string, args ...string) ([]byte, error) {
	if DryRun {
		printDryRun(name, args...)
		return nil, ErrDryRun
	}
	Echo(name, args...)
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// Inherit runs a command wired to the current terminal, for interactive flows
// such as `gcloud auth login`. It blocks until the command exits.
func Inherit(name string, args ...string) error {
	if DryRun {
		printDryRun(name, args...)
		return ErrDryRun
	}
	Echo(name, args...)
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
