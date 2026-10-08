package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func available(name string) (string, error) { return name, nil }

func simulate(c command) result {
	switch {
	case c.program == codexAgent:
		return result{stdout: `{"decision":"forbidden"}`}
	case len(c.args) > 2 && c.args[1] == "guard":
		agent := c.args[len(c.args)-1]
		if strings.Contains(c.input, "delete") {
			if agent == cursorAgent {
				return result{stdout: `{"permission":"deny"}`}
			}
			return result{code: 2, stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny"}}`}
		}
		if agent == cursorAgent {
			return result{stdout: `{"permission":"allow"}`}
		}
	}
	return result{}
}

func TestRun(t *testing.T) {
	var calls []command
	var out bytes.Buffer
	code := run(&out, func(c command) result {
		calls = append(calls, c)
		return simulate(c)
	}, available)
	if code != 0 || strings.Count(out.String(), pass) != 9 || len(calls) != 11 {
		t.Fatalf("code=%d calls=%d\n%s", code, len(calls), out.String())
	}
	home := calls[2].dir
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("temporary home survived: %v", err)
	}
	if calls[0].args[0] != "build" || calls[1].args[0] != "extension" {
		t.Fatalf("missing build or extension installation: %v", calls[:2])
	}
	if strings.Join(calls[2].args, " ") != "board agent install --agent all" {
		t.Fatalf("installer command: %v", calls[2].args)
	}
	checkIsolation(t, home, calls[2:])
}

func checkIsolation(t *testing.T, home string, calls []command) {
	t.Helper()
	for _, c := range calls {
		if !strings.Contains(strings.Join(c.env, "\n"), "HOME="+home) {
			t.Fatalf("missing isolated HOME: %v", c.args)
		}
		if c.program == "gh" && c.args[0] != "board" {
			t.Fatalf("raw gh command reached executor: %v", c.args)
		}
		if c.program == codexAgent {
			want := filepath.Join(home, ".codex", "rules", "gh-board.rules")
			if c.args[3] != want || strings.Join(c.args[4:], " ") != destructive {
				t.Fatalf("execpolicy command: %v", c.args)
			}
		}
	}
}

func TestRunSetupFailures(t *testing.T) {
	for _, index := range []int{0, 1, 2} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			var out bytes.Buffer
			calls := 0
			code := run(&out, func(c command) result {
				calls++
				if calls == index+1 {
					return result{code: 1, stderr: "setup failed"}
				}
				return simulate(c)
			}, available)
			if code != 1 || calls != index+1 || !strings.Contains(out.String(), "setup failed") {
				t.Fatalf("code=%d calls=%d: %s", code, calls, out.String())
			}
		})
	}
}

func TestPrepareMissingTools(t *testing.T) {
	for _, name := range []string{"go", "gh"} {
		s := suite{home: t.TempDir(), execute: simulate, lookup: func(tool string) (string, error) {
			if tool == name {
				return "", exec.ErrNotFound
			}
			return tool, nil
		}}
		if err := s.prepare(); !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestPrepareInvalidHome(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := suite{home: file}
	if err := s.prepare(); err == nil {
		t.Fatal("accepted a regular file as HOME")
	}
}
