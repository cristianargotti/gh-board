package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type command struct {
	program string
	args    []string
	dir     string
	env     []string
	input   string
}

type result struct {
	stdout, stderr string
	code           int
	err            error
}

type executor func(command) result

type locator func(string) (string, error)

func runCommand(c command) result {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Arguments are fixed by this tool; hook input is never passed to a shell.
	cmd := exec.CommandContext(ctx, c.program, c.args...) //nolint:gosec // Executes only the build, installer, and policy evaluators.
	cmd.Dir, cmd.Env, cmd.Stdin = c.dir, c.env, strings.NewReader(c.input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		r.code = exit.ExitCode()
	} else {
		r.err = err
	}
	return r
}

func success(r result) error {
	if r.err != nil {
		return r.err
	}
	if r.code != 0 {
		return fmt.Errorf("exit %d: %s", r.code, strings.TrimSpace(r.stdout+"\n"+r.stderr))
	}
	return nil
}
