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

// Output runs a command, echoing it first, and returns its stdout. On failure
// the command's stderr is folded into the returned error (see runCaptured).
func Output(name string, args ...string) ([]byte, error) {
	if DryRun {
		printDryRun(name, args...)
		return nil, ErrDryRun
	}
	Echo(name, args...)
	return runCaptured(exec.Command(name, args...))
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
	return runCaptured(cmd)
}

// runCaptured runs cmd and returns its stdout, folding the command's stderr into
// the error when it fails. The caller has already echoed the command (see Echo),
// so transparency is preserved; this only changes where a failure's stderr goes
// — into the returned error rather than only to this process's stderr. That
// matters for the MCP server: an agent sees a tool's error but never the
// server's stderr, so without this it would get a bare "exit status 1" with no
// hint of the real cause (an expired token, a missing instance, a denied call).
func runCaptured(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return out, fmt.Errorf("%w: %s", err, msg)
		}
	}
	return out, err
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
