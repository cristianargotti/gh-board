package commands

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

var doctorInstallCommands = [][]string{
	{"agent", "install", "--agent", "all"},
	{"guard", "install", "--agent", "all", "--strict"},
	{"guard", "uninstall", "--agent", "all", "--strict"},
}

func TestDoctorVerifiesInstalledAgents(t *testing.T) {
	env := autoNewTestEnv(t)
	env.seedForeign(t)
	for _, command := range doctorInstallCommands {
		if err := env.run(t, command...); err != nil {
			t.Fatal(err)
		}
		assertDoctorArtifacts(t, env.deps)
	}
	path := autoSkillPath(env.home, guard.AgentCursor)
	autoWriteTestFile(t, path, []byte("changed skill"))
	report := &readDoctorReport{}
	readDoctorArtifactsCheck(env.deps, report)
	for _, artifact := range report.Artifacts {
		if artifact.Path == path && !artifact.OK {
			return
		}
	}
	t.Fatalf("doctor did not report a modified skill: %+v", report)
}

func assertDoctorArtifacts(t *testing.T, deps *Deps) {
	t.Helper()
	report := &readDoctorReport{}
	readDoctorArtifactsCheck(deps, report)
	if len(report.Artifacts) == 0 {
		t.Fatalf("no installed artifacts verified: %+v", report)
	}
	for _, artifact := range report.Artifacts {
		if !artifact.OK {
			t.Errorf("installed artifact failed verification: %+v", artifact)
		}
	}
}

// The hooks name the binary that installed them; a doctor run from another
// binary reports every hook, in the POSIX and the Windows form alike.
func TestDoctorComparesHookBinaries(t *testing.T) {
	env := autoNewTestEnv(t)
	if err := env.run(t, "agent", "install", "--agent", "all"); err != nil {
		t.Fatal(err)
	}
	report := &readDoctorReport{}
	readDoctorHooksCheck(env.deps, report)
	if len(report.Hooks) != 3 || len(report.Problems) != 0 {
		t.Fatalf("hooks of the installing binary: %+v, problems %v", report.Hooks, report.Problems)
	}
	for _, h := range report.Hooks {
		if !h.OK || h.Binary == "" {
			t.Fatalf("hook not verified: %+v", h)
		}
	}
	readStubExecutable(t, filepath.Join(t.TempDir(), "bin", "gh-board"))
	moved := &readDoctorReport{}
	readDoctorHooksCheck(env.deps, moved)
	if len(moved.Hooks) != 3 || len(moved.Problems) != 3 {
		t.Fatalf("hooks of a moved binary: %+v, problems %v", moved.Hooks, moved.Problems)
	}
	for _, h := range moved.Hooks {
		if h.OK || !strings.Contains(h.Reason, "this binary is") {
			t.Fatalf("moved binary not reported: %+v", h)
		}
	}
	empty := &readDoctorReport{}
	readDoctorHooksCheck(&Deps{Dirs: config.Paths(t.TempDir())}, empty)
	if len(empty.Hooks) != 0 || !strings.Contains(empty.HooksNote, "no agent hooks recorded") {
		t.Fatalf("no receipts: %+v", empty)
	}
}

func TestCommandStrings(t *testing.T) {
	var out []string
	autoCommandStrings(map[string]any{
		"hooks":   map[string]any{"PreToolUse": []any{map[string]any{"hooks": []any{map[string]any{"command": "a", "type": "command"}}}}},
		"version": float64(1), "command": "b", "other": []any{"c"},
	}, &out)
	sort.Strings(out)
	if strings.Join(out, ",") != "a,b" {
		t.Fatalf("commands = %v", out)
	}
}
