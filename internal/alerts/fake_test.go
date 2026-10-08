package alerts_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// fakeRunner records every command and answers LookPath from a fixed map,
// so that no notifier runs and no scheduler is registered by the tests.
type fakeRunner struct {
	paths   map[string]string
	respond func(cmd alerts.Command) ([]byte, error)
	calls   []alerts.Command
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if p, ok := f.paths[file]; ok {
		return p, nil
	}
	return "", fmt.Errorf("%s: executable file not found", file)
}

func (f *fakeRunner) Run(_ context.Context, cmd alerts.Command) ([]byte, error) {
	f.calls = append(f.calls, cmd)
	if f.respond == nil {
		return nil, nil
	}
	return f.respond(cmd)
}

// lines renders the recorded commands one per line.
func (f *fakeRunner) lines() []string {
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.String())
	}
	return out
}

// failing answers every command with an error carrying the output.
func failing(output string) func(alerts.Command) ([]byte, error) {
	return func(cmd alerts.Command) ([]byte, error) {
		return []byte(output), fmt.Errorf("%s: exit status 1", cmd.Name)
	}
}

var (
	fxNow     = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	fxEarlier = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	fxProject = domain.ProjectRef{Owner: "acme", Number: 7}
	fxOther   = domain.ProjectRef{Owner: "other", Number: 1}
)

// alert builds an item alert with sanitized identifiers.
func alert(rule string, sev domain.Severity, n int, msg string) domain.Alert {
	return domain.Alert{
		RuleID: rule, Severity: sev, Message: msg, FirstSeen: fxNow,
		Item: domain.ItemRef{
			NodeID: fmt.Sprintf("I_%d", n), Repository: "acme/team-docs", Number: n,
			Title: fmt.Sprintf("Item %d", n), URL: fmt.Sprintf("https://github.com/acme/team-docs/issues/%d", n),
		},
	}
}

// boardAlert builds a board-level alert, which carries no item.
func boardAlert(rule string, sev domain.Severity, msg string) domain.Alert {
	return domain.Alert{RuleID: rule, Severity: sev, Message: msg, FirstSeen: fxNow}
}

// seen sets the first seen instant of an alert.
func seen(a domain.Alert, at time.Time) domain.Alert {
	a.FirstSeen = at
	return a
}

func stateOf(project domain.ProjectRef, list ...domain.Alert) domain.AlertState {
	return domain.AlertState{GeneratedAt: fxNow, Project: project, Summary: "resumo", Alerts: list}
}

// fakeBinary writes an executable file under dir and returns its resolved
// path, the one the schedulers reference.
func fakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "bin", "gh-board")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // a fixture, never run
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// resolvedHome is the temporary home with its symlinks followed, so that
// expectations match what the installers print on every platform.
func resolvedHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return home
}

// wantError checks the expected error text of a case and reports whether
// the case ends there; an empty text means no error is expected.
func wantError(t *testing.T, err error, text string) bool {
	t.Helper()
	if text == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return false
	}
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want %q", err, text)
	}
	return true
}

// assertLines compares recorded commands with the expected ones.
func assertLines(t *testing.T, got []string, want string) {
	t.Helper()
	if joined := strings.Join(got, "\n"); joined != want {
		t.Fatalf("commands:\n%s\nwant:\n%s", joined, want)
	}
}

// assertOutput compares what a command printed.
func assertOutput(t *testing.T, out *bytes.Buffer, want string) {
	t.Helper()
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
}

// assertPerm checks the owner-only permission where the OS enforces it.
func assertPerm(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("%s: permissions = %v, %v", path, info.Mode().Perm(), err)
	}
}
