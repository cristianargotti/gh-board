package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

var autoSetupCases = []struct {
	name  string
	agent guard.Agent
	scope autoScope
}{
	{"claude user", guard.AgentClaude, autoScopeUser},
	{"claude project", guard.AgentClaude, autoScopeProject},
	{"cursor user", guard.AgentCursor, autoScopeUser},
	{"cursor project", guard.AgentCursor, autoScopeProject},
	{"codex user", guard.AgentCodex, autoScopeUser},
	{"codex project", guard.AgentCodex, autoScopeProject},
}

// TestAutoAgentInstallCycle installs, reinstalls and uninstalls every
// agent in both scopes over foreign content, checking idempotency and
// exact removal.
func TestAutoAgentInstallCycle(t *testing.T) {
	for _, tc := range autoSetupCases {
		t.Run(tc.name, func(t *testing.T) {
			env := autoNewTestEnv(t)
			env.seedForeign(t)
			args := []string{"--agent", string(tc.agent), "--scope", string(tc.scope)}
			if err := env.run(t, append([]string{"agent", "install"}, args...)...); err != nil {
				t.Fatalf("install: %v\n%s", err, env.errOut.String())
			}
			if !strings.Contains(env.out.String(), "+++ ") {
				t.Fatalf("install printed no diff:\n%s", env.out.String())
			}
			autoExpectInstalled(t, env, tc.agent, tc.scope)
			if err := env.run(t, append([]string{"--json", "agent", "install"}, args...)...); err != nil {
				t.Fatalf("reinstall: %v", err)
			}
			autoExpectAllActions(t, env.report(t), autoActionUnchanged)
			if err := env.run(t, append([]string{"agent", "uninstall"}, args...)...); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			autoExpectRemoved(t, env, tc.agent, tc.scope)
		})
	}
}

func autoExpectAllActions(t *testing.T, report autoSetupReport, want string) {
	t.Helper()
	for path, action := range report.actions() {
		if action != want {
			t.Errorf("%s: action %q, want %q", path, action, want)
		}
	}
}

// autoExpectInstalled checks every planned artifact against the disk and
// the receipt.
func autoExpectInstalled(t *testing.T, env *autoTestEnv, agent guard.Agent, scope autoScope) {
	t.Helper()
	for _, a := range env.plan(t, agent, scope, false) {
		autoExpectArtifactInstalled(t, a)
	}
	receipt, found, err := autoReadReceipt(autoReceiptPath(env.deps.Dirs, agent, scope))
	if err != nil || !found {
		t.Fatalf("receipt: found=%v err=%v", found, err)
	}
	if receipt.Agent != agent || receipt.Scope != scope || len(receipt.Entries) == 0 {
		t.Fatalf("receipt = %+v", receipt)
	}
	autoExpectForeignKept(t, env, agent, scope)
}

func autoExpectArtifactInstalled(t *testing.T, a autoArtifact) {
	t.Helper()
	data, err := os.ReadFile(a.Path)
	if err != nil {
		t.Fatalf("%s: %v", a.Path, err)
	}
	switch a.Type {
	case autoTypeFile:
		if !bytes.Equal(data, a.Content) {
			t.Errorf("%s: content differs", a.Path)
		}
	case autoTypeJSON:
		if delta := autoJSONDelta(autoReadTestJSON(t, a.Path), a.Fragment); delta != nil {
			t.Errorf("%s: fragment missing: %v", a.Path, delta)
		}
	case autoTypeBlock:
		if !bytes.Contains(data, autoBlock(autoBlockBody(a.Content))) || !bytes.HasPrefix(data, autoForeignAgents) {
			t.Errorf("%s: block or original text missing:\n%s", a.Path, data)
		}
	}
}

// autoExpectForeignKept checks the foreign entries survived the merge.
func autoExpectForeignKept(t *testing.T, env *autoTestEnv, agent guard.Agent, scope autoScope) {
	t.Helper()
	path, foreign := autoForeignFile(env, agent, scope)
	obj := autoReadTestJSON(t, path)
	if autoJSONDelta(obj, foreign) != nil {
		t.Errorf("%s lost foreign content: %v", path, obj)
	}
	if agent == guard.AgentClaude && obj["model"] != "opus" {
		t.Errorf("%s lost the model key: %v", path, obj)
	}
}

// autoForeignFile is the JSON file of an agent and the foreign content it
// was seeded with.
func autoForeignFile(env *autoTestEnv, agent guard.Agent, scope autoScope) (string, map[string]any) {
	switch agent {
	case guard.AgentCursor:
		return filepath.Join(env.home, autoCursorDir, autoHooksFile), autoForeignCursorHooks
	case guard.AgentCodex:
		return filepath.Join(env.home, autoCodexDir, autoHooksFile), autoForeignCodexHooks
	}
	if scope == autoScopeProject {
		return filepath.Join(env.project, autoClaudeDir, autoClaudeSettings), autoForeignSettings
	}
	return filepath.Join(env.home, autoClaudeDir, autoClaudeSettings), autoForeignSettings
}

// autoExpectRemoved checks that uninstall left exactly the foreign content.
func autoExpectRemoved(t *testing.T, env *autoTestEnv, agent guard.Agent, scope autoScope) {
	t.Helper()
	for _, a := range env.plan(t, agent, scope, false) {
		autoExpectArtifactRemoved(t, a)
	}
	if autoExists(autoReceiptPath(env.deps.Dirs, agent, scope)) {
		t.Error("receipt still exists")
	}
}

func autoExpectArtifactRemoved(t *testing.T, a autoArtifact) {
	t.Helper()
	switch a.Type {
	case autoTypeFile:
		if autoExists(a.Path) {
			t.Errorf("%s still exists", a.Path)
		}
		if a.OwnDir && autoExists(filepath.Dir(a.Path)) {
			t.Errorf("%s directory still exists", filepath.Dir(a.Path))
		}
	case autoTypeJSON:
		if got := autoReadTestJSON(t, a.Path); !reflect.DeepEqual(got, autoForeignOf(a.Path)) {
			t.Errorf("%s after uninstall = %v", a.Path, got)
		}
	case autoTypeBlock:
		data, err := os.ReadFile(a.Path)
		if err != nil || !bytes.Equal(data, autoForeignAgents) {
			t.Errorf("%s after uninstall = %q (%v)", a.Path, data, err)
		}
	}
}

// autoForeignOf returns the foreign object seeded at a JSON path.
func autoForeignOf(path string) map[string]any {
	switch filepath.Base(filepath.Dir(path)) {
	case autoCursorDir:
		return autoForeignCursorHooks
	case autoCodexDir:
		return autoForeignCodexHooks
	}
	return autoForeignSettings
}

func TestAutoAgentInstallSpotChecks(t *testing.T) {
	env := autoNewTestEnv(t)
	env.seedForeign(t)
	if err := env.run(t, "agent", "install", "--scope", "project"); err != nil {
		t.Fatalf("install all: %v", err)
	}
	settings := autoReadTestJSON(t, filepath.Join(env.home, autoClaudeDir, autoClaudeSettings))
	if settings["model"] != "opus" || settings["enabledPlugins"] != nil {
		t.Fatalf("user settings touched by project scope: %v", settings)
	}
	project := autoReadTestJSON(t, filepath.Join(env.project, autoClaudeDir, autoClaudeSettings))
	deny := project["permissions"].(map[string]any)["deny"].([]any)
	if !autoContainsEntry(deny, "Bash(rm -rf /)") || !autoContainsEntry(deny, "Bash(gh project delete *)") {
		t.Fatalf("deny = %v", deny)
	}
	if _, ok := project["enabledPlugins"].(map[string]any)["gh-board@gh-board"]; !ok {
		t.Fatalf("plugin pointer missing: %v", project)
	}
	cursor := autoReadTestJSON(t, filepath.Join(env.home, autoCursorDir, autoHooksFile))
	if hooks := cursor["hooks"].(map[string]any)["beforeShellExecution"].([]any); len(hooks) != 2 {
		t.Fatalf("cursor hooks = %v", hooks)
	}
	skill, _ := autoAsset(autoAssetSkill, "")
	for _, agent := range []guard.Agent{guard.AgentCursor, guard.AgentCodex} {
		if data, err := os.ReadFile(autoSkillPath(env.home, agent)); err != nil || !bytes.Equal(data, skill) {
			t.Fatalf("%s skill: %v", agent, err)
		}
	}
	agents, _ := os.ReadFile(filepath.Join(env.project, autoAgentsFile))
	if !bytes.HasPrefix(agents, autoForeignAgents) || bytes.Count(agents, []byte(autoBlockBegin)) != 1 {
		t.Fatalf("AGENTS.md = %q", agents)
	}
	if !autoExists(filepath.Join(env.project, autoCursorDir, autoRulesDir, autoCursorRuleFile)) {
		t.Fatal("cursor rule missing")
	}
	if !autoExists(filepath.Join(env.home, autoCodexDir, autoRulesDir, autoCodexRulesFile)) {
		t.Fatal("codex rules missing")
	}
}
