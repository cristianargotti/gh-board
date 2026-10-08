// Command acceptance checks the installed guards without executing their input.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() { os.Exit(run(os.Stdout, runCommand, exec.LookPath)) }

func run(out io.Writer, execute executor, lookup locator) int {
	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(out, err)
		return 1
	}
	home, err := os.MkdirTemp("", "gh-board-acceptance-")
	if err != nil {
		_, _ = fmt.Fprintln(out, err)
		return 1
	}
	defer os.RemoveAll(home) //nolint:errcheck // The disposable home is removed on every normal exit.
	s := suite{root: root, home: home, execute: execute, lookup: lookup}
	if err := s.prepare(); err != nil {
		s.rows = append(s.rows, row{"setup", fail, err.Error()})
	} else {
		s.checkGuards()
		s.checkOptional()
	}
	return report(out, s.rows)
}

type suite struct {
	root, home string
	gh         string
	env        []string
	rows       []row
	execute    executor
	lookup     locator
}

func (s *suite) prepare() error {
	binDir := filepath.Join(s.home, "gh-board")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		return err
	}
	binary := filepath.Join(binDir, "gh-board")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	goPath, err := s.lookup("go")
	if err != nil {
		return err
	}
	if err := success(s.execute(command{program: goPath, args: []string{"build", "-o", binary, "./cmd/gh-board"}, dir: s.root})); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	s.env = isolatedEnv(os.Environ(), s.home)
	s.gh, err = s.lookup("gh")
	if err != nil {
		return fmt.Errorf("GitHub CLI is required: %w", err)
	}
	if err := success(s.execute(command{program: s.gh, args: []string{"extension", "install", "."}, dir: binDir, env: s.env})); err != nil {
		return fmt.Errorf("install temporary extension: %w", err)
	}
	if err := success(s.board("", "agent", "install", "--agent", "all")); err != nil {
		return fmt.Errorf("install agents: %w", err)
	}
	s.rows = append(s.rows, row{"agent install all", pass, "temporary HOME"})
	return nil
}

func (s *suite) board(input string, args ...string) result {
	return s.execute(command{program: s.gh, args: append([]string{"board"}, args...), dir: s.home, env: s.env, input: input})
}
