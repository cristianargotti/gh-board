package alerts

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Command is one executable invocation. Name is resolved on the PATH by
// the runner, Args travel as data and never through a shell, Stdin feeds
// the process when set.
type Command struct {
	Name  string
	Args  []string
	Stdin []byte
}

// String renders the command the way a dry run prints it, quoting the
// arguments a shell would split.
func (c Command) String() string {
	parts := make([]string, 0, len(c.Args)+1)
	parts = append(parts, c.Name)
	for _, arg := range c.Args {
		if arg == "" || strings.ContainsAny(arg, " \t\n\"'") {
			arg = strconv.Quote(arg)
		}
		parts = append(parts, arg)
	}
	return strings.Join(parts, " ")
}

// Runner launches executables. ExecRunner is the real one; tests inject a
// recorder so that no notifier runs and no scheduler is registered.
type Runner interface {
	// LookPath resolves an executable name on the PATH; an error means
	// the program is absent.
	LookPath(file string) (string, error)
	// Run executes the command and returns its combined output, with an
	// error that carries the output when the program fails.
	Run(ctx context.Context, cmd Command) ([]byte, error)
}

// ExecRunner runs commands through os/exec.
type ExecRunner struct{}

// LookPath resolves file on the PATH.
func (ExecRunner) LookPath(file string) (string, error) { return exec.LookPath(file) }

// Run executes the command and waits for it.
func (ExecRunner) Run(ctx context.Context, cmd Command) ([]byte, error) {
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...) //nolint:gosec // the names are fixed by the kit and the arguments travel as data
	if cmd.Stdin != nil {
		c.Stdin = bytes.NewReader(cmd.Stdin)
	}
	out, err := c.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", cmd.Name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
