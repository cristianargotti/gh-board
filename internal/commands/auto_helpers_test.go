package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoFixedClock is the clock of the automation tests.
type autoFixedClock struct{ t time.Time }

func (c autoFixedClock) Now() time.Time { return c.t }

var autoTestNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// autoTestEnv is a temporary home, kit directory and working directory
// with the deps of a command run.
type autoTestEnv struct {
	home    string
	project string
	deps    *Deps
	out     *bytes.Buffer
	errOut  *bytes.Buffer
}

// autoNewTestEnv points HOME and the working directory at temporary
// directories, so that no test touches the real agent configuration.
func autoNewTestEnv(t *testing.T) *autoTestEnv {
	t.Helper()
	root := t.TempDir()
	home, project := filepath.Join(root, "home"), filepath.Join(root, "project")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfig, "")
	t.Chdir(project)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	deps := &Deps{
		Out: out, Err: errOut, Version: "test",
		Dirs:  config.Paths(filepath.Join(root, "kit")),
		Clock: autoFixedClock{autoTestNow},
	}
	return &autoTestEnv{home: home, project: wd, deps: deps, out: out, errOut: errOut}
}

// run executes a command line against the deps.
func (e *autoTestEnv) run(t *testing.T, args ...string) error {
	t.Helper()
	e.out.Reset()
	e.errOut.Reset()
	e.deps.Flags = GlobalFlags{}
	return Execute(context.Background(), args, e.deps)
}

// roots are the roots a scope resolves to in this environment.
func (e *autoTestEnv) roots(scope autoScope) autoRoots {
	roots := autoRoots{Home: e.home}
	if scope == autoScopeProject {
		roots.Project = e.project
	}
	return roots
}

// Foreign content the installers must preserve.
var (
	autoForeignSettings = map[string]any{
		"model": "opus",
		"permissions": map[string]any{
			"allow": []any{"Bash(ls)"},
			"deny":  []any{"Bash(rm -rf /)"},
		},
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{"matcher": "Write", "hooks": []any{}}},
		},
	}
	autoForeignCursorHooks = map[string]any{
		"version": float64(1),
		"hooks": map[string]any{
			"beforeShellExecution": []any{map[string]any{"command": "other-guard"}},
		},
	}
	autoForeignCodexHooks = map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{"matcher": "Write", "hooks": []any{}}},
		},
	}
	autoForeignAgents = []byte("# Team\n\nRules of the team.\n")
)

// seedForeign writes the foreign content of every agent in both scopes.
func (e *autoTestEnv) seedForeign(t *testing.T) {
	t.Helper()
	e.writeJSON(t, filepath.Join(e.home, autoClaudeDir, autoClaudeSettings), autoForeignSettings)
	e.writeJSON(t, filepath.Join(e.project, autoClaudeDir, autoClaudeSettings), autoForeignSettings)
	e.writeJSON(t, filepath.Join(e.home, autoCursorDir, autoHooksFile), autoForeignCursorHooks)
	e.writeJSON(t, filepath.Join(e.home, autoCodexDir, autoHooksFile), autoForeignCodexHooks)
	autoWriteTestFile(t, filepath.Join(e.project, autoAgentsFile), autoForeignAgents)
}

func (e *autoTestEnv) writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := autoJSONEncode(v.(map[string]any))
	if err != nil {
		t.Fatal(err)
	}
	autoWriteTestFile(t, path, data)
}

func autoWriteTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func autoReadTestJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	obj, err := autoJSONObject(data, path)
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func autoExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// report decodes the --json report of the last run.
func (e *autoTestEnv) report(t *testing.T) autoSetupReport {
	t.Helper()
	var r autoSetupReport
	if err := json.Unmarshal(e.out.Bytes(), &r); err != nil {
		t.Fatalf("report: %v\n%s", err, e.out.String())
	}
	return r
}

// actions maps every file of the report to its action.
func (r autoSetupReport) actions() map[string]string {
	out := map[string]string{}
	for _, a := range r.Agents {
		for _, f := range a.Files {
			out[f.Path] = f.Action
		}
	}
	return out
}

// plan lists the artifacts of an agent in this environment.
func (e *autoTestEnv) plan(t *testing.T, agent guard.Agent, scope autoScope, strict bool) []autoArtifact {
	t.Helper()
	artifacts, err := autoPlanArtifacts(agent, scope, strict, e.roots(scope))
	if err != nil {
		t.Fatal(err)
	}
	return artifacts
}
