package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

func TestAutoDryRunWritesNothing(t *testing.T) {
	env := autoNewTestEnv(t)
	env.seedForeign(t)
	hooks := filepath.Join(env.home, autoCursorDir, autoHooksFile)
	if err := env.run(t, "--dry-run", "agent", "install", "--agent", "cursor"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(env.out.String(), "+++ "+hooks) || !strings.Contains(env.out.String(), "dry run") {
		t.Fatalf("output:\n%s", env.out.String())
	}
	if autoExists(autoSkillPath(env.home, guard.AgentCursor)) {
		t.Fatal("dry run wrote the skill")
	}
	if got := autoReadTestJSON(t, hooks); !reflect.DeepEqual(got, autoForeignCursorHooks) {
		t.Fatalf("dry run changed hooks: %v", got)
	}
	if autoExists(autoReceiptPath(env.deps.Dirs, guard.AgentCursor, autoScopeUser)) {
		t.Fatal("dry run wrote a receipt")
	}
	if err := env.run(t, "agent", "install", "--agent", "cursor"); err != nil {
		t.Fatal(err)
	}
	if err := env.run(t, "--dry-run", "agent", "uninstall", "--agent", "cursor"); err != nil {
		t.Fatal(err)
	}
	if !autoExists(autoSkillPath(env.home, guard.AgentCursor)) {
		t.Fatal("dry run uninstall removed the skill")
	}
}

func TestAutoJSONReportKeepsStdoutClean(t *testing.T) {
	env := autoNewTestEnv(t)
	if err := env.run(t, "--json", "agent", "install", "--agent", "codex"); err != nil {
		t.Fatal(err)
	}
	report := env.report(t)
	if report.Command != "install" || len(report.Agents) != 1 || report.Agents[0].Agent != guard.AgentCodex {
		t.Fatalf("report = %+v", report)
	}
	for path, action := range report.actions() {
		if action != autoActionCreated {
			t.Errorf("%s: %s", path, action)
		}
	}
	if !strings.Contains(env.errOut.String(), "+++ ") {
		t.Fatalf("diff missing from stderr:\n%s", env.errOut.String())
	}
}

func TestAutoGuardLayerAlone(t *testing.T) {
	env := autoNewTestEnv(t)
	skill := autoSkillPath(env.home, guard.AgentCodex)
	rules := filepath.Join(env.home, autoCodexDir, autoRulesDir, autoCodexRulesFile)
	receiptPath := autoReceiptPath(env.deps.Dirs, guard.AgentCodex, autoScopeUser)
	if err := env.run(t, "guard", "install", "--agent", "codex"); err != nil {
		t.Fatal(err)
	}
	if !autoExists(rules) || autoExists(skill) {
		t.Fatal("guard install must write the rules and not the skill")
	}
	if err := env.run(t, "agent", "install", "--agent", "codex"); err != nil {
		t.Fatal(err)
	}
	if !autoExists(skill) {
		t.Fatal("agent install must add the skill")
	}
	if err := env.run(t, "guard", "uninstall", "--agent", "codex"); err != nil {
		t.Fatal(err)
	}
	if autoExists(rules) || !autoExists(skill) {
		t.Fatal("guard uninstall must remove the rules and keep the skill")
	}
	receipt, found, err := autoReadReceipt(receiptPath)
	if err != nil || !found || len(receipt.Entries) != 1 || receipt.Entries[0].Kind != autoKindSkill {
		t.Fatalf("receipt after guard uninstall: %+v (found %v, err %v)", receipt, found, err)
	}
	if err := env.run(t, "agent", "uninstall", "--agent", "codex"); err != nil {
		t.Fatal(err)
	}
	if autoExists(skill) || autoExists(receiptPath) {
		t.Fatal("agent uninstall must remove the skill and the receipt")
	}
}

func TestAutoUninstallWithoutReceipt(t *testing.T) {
	env := autoNewTestEnv(t)
	env.seedForeign(t)
	if err := env.run(t, "agent", "install", "--agent", "cursor", "--scope", "project"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(autoReceiptPath(env.deps.Dirs, guard.AgentCursor, autoScopeProject)); err != nil {
		t.Fatal(err)
	}
	if err := env.run(t, "--json", "agent", "uninstall", "--agent", "cursor", "--scope", "project"); err != nil {
		t.Fatal(err)
	}
	autoExpectRemoved(t, env, guard.AgentCursor, autoScopeProject)
	if err := env.run(t, "--json", "agent", "uninstall", "--agent", "claude"); err != nil {
		t.Fatal(err)
	}
	// Files the kit never wrote (the MCP entry of an agent that was not
	// installed) are reported absent; everything else is kept.
	for path, action := range env.report(t).actions() {
		if action != autoActionKept && action != autoActionAbsent {
			t.Errorf("%s: %s", path, action)
		}
	}
}

var autoSetupErrorCases = []struct {
	name string
	args []string
}{
	{"bad agent", []string{"agent", "install", "--agent", "copilot"}},
	{"bad scope", []string{"agent", "install", "--scope", "global"}},
	{"bad format", []string{"--format", "xml", "agent", "install", "--agent", "codex"}},
	{"positional", []string{"guard", "install", "codex"}},
}

func TestAutoSetupUsageErrors(t *testing.T) {
	for _, tc := range autoSetupErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			env := autoNewTestEnv(t)
			err := env.run(t, tc.args...)
			if domain.CodeOf(err) != domain.ExitUsage {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAutoInstallRefusesDamagedSettings(t *testing.T) {
	env := autoNewTestEnv(t)
	path := filepath.Join(env.home, autoClaudeDir, autoClaudeSettings)
	autoWriteTestFile(t, path, []byte("{not json"))
	err := env.run(t, "agent", "install", "--agent", "claude")
	if err == nil || !strings.Contains(err.Error(), "not a JSON object") {
		t.Fatalf("err = %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Fatalf("file changed: %q", data)
	}
}

func TestAutoStrictInstallUnionsTheReceipt(t *testing.T) {
	env := autoNewTestEnv(t)
	path := filepath.Join(env.home, autoClaudeDir, autoClaudeSettings)
	if err := env.run(t, "agent", "install", "--agent", "claude"); err != nil {
		t.Fatal(err)
	}
	if err := env.run(t, "agent", "install", "--agent", "claude", "--strict"); err != nil {
		t.Fatal(err)
	}
	deny := autoReadTestJSON(t, path)["permissions"].(map[string]any)["deny"].([]any)
	if !autoContainsEntry(deny, "Bash(sh -c *)") {
		t.Fatalf("strict rule missing: %v", deny)
	}
	receipt, _, err := autoReadReceipt(autoReceiptPath(env.deps.Dirs, guard.AgentClaude, autoScopeUser))
	if err != nil || !receipt.Strict {
		t.Fatalf("receipt = %+v, %v", receipt, err)
	}
	if err := env.run(t, "agent", "uninstall", "--agent", "claude"); err != nil {
		t.Fatal(err)
	}
	if autoExists(path) {
		t.Fatal("a settings file the kit created must go away when empty")
	}
}

func TestAutoConflictsAreNoted(t *testing.T) {
	env := autoNewTestEnv(t)
	path := filepath.Join(env.home, autoClaudeDir, autoClaudeSettings)
	env.writeJSON(t, path, map[string]any{"enabledPlugins": map[string]any{"gh-board@gh-board": false}})
	if err := env.run(t, "--json", "agent", "install", "--agent", "claude"); err != nil {
		t.Fatal(err)
	}
	var noted bool
	for _, f := range env.report(t).Agents[0].Files {
		for _, n := range f.Notes {
			noted = noted || strings.Contains(n, "enabledPlugins.gh-board@gh-board already set")
		}
	}
	if !noted {
		t.Fatalf("conflict not noted:\n%s", env.out.String())
	}
	if autoReadTestJSON(t, path)["enabledPlugins"].(map[string]any)["gh-board@gh-board"] != false {
		t.Fatal("the user's value was overwritten")
	}
}

func TestAutoParentsPrintHelp(t *testing.T) {
	env := autoNewTestEnv(t)
	for _, parent := range []string{"agent", "guard"} {
		if err := env.run(t, parent); err != nil || !strings.Contains(env.out.String(), "install") {
			t.Fatalf("%s: %v\n%s", parent, err, env.out.String())
		}
	}
}

func TestAutoResolveRootsWithoutHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("HOME is not the home variable on Windows")
	}
	t.Setenv("HOME", "")
	if _, err := autoResolveRoots(autoScopeUser); err == nil {
		t.Fatal("expected an error without HOME")
	}
}

var autoMarketplaceCases = []struct {
	module string
	repo   string
	ok     bool
}{
	{"github.com/cristianargotti/gh-board", "cristianargotti/gh-board", true},
	{"github.com/acme/gh-board/v2", "acme/gh-board", true},
	{"gitlab.com/x/y", "", false},
	{"github.com//y", "", false},
	{"", "", false},
}

func TestAutoMarketplace(t *testing.T) {
	for _, tc := range autoMarketplaceCases {
		spec, err := autoMarketplace(tc.module)
		if (err == nil) != tc.ok || spec.Repo != tc.repo {
			t.Errorf("%q: spec %+v err %v", tc.module, spec, err)
		}
	}
	if _, err := autoMarketplace(autoModulePath()); err != nil {
		t.Fatalf("the test binary module: %v", err)
	}
	if autoBashRule("Bash(x)") != "Bash(x)" || autoBashRule("x *") != "Bash(x *)" {
		t.Fatal("autoBashRule")
	}
}
