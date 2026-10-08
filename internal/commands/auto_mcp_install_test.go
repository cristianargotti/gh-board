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

var autoMCPCases = []struct {
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

// Foreign MCP content the installers must preserve.
var (
	autoForeignMCPJSON = map[string]any{
		"numStartups": float64(3),
		"mcpServers":  map[string]any{"other": map[string]any{"type": "stdio", "command": "other", "args": []any{}}},
	}
	autoForeignCodexConfig = []byte("model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"other\"\nargs = []\n")
)

// seedForeignMCP writes the foreign MCP files of every agent in both scopes.
func (e *autoTestEnv) seedForeignMCP(t *testing.T) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(e.home, autoClaudeUserMCP), filepath.Join(e.project, autoClaudeProjectMCP),
		filepath.Join(e.home, autoCursorDir, autoCursorMCPFile), filepath.Join(e.project, autoCursorDir, autoCursorMCPFile),
	} {
		e.writeJSON(t, path, autoForeignMCPJSON)
	}
	autoWriteTestFile(t, filepath.Join(e.home, autoCodexDir, autoCodexConfigFile), autoForeignCodexConfig)
}

// mcpEntry is the planned MCP artifact of an agent in this environment.
func (e *autoTestEnv) mcpEntry(t *testing.T, agent guard.Agent, scope autoScope) autoArtifact {
	t.Helper()
	artifacts, err := autoPlanMCP(agent, scope, e.roots(scope))
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("plan: %v %v", artifacts, err)
	}
	return artifacts[0]
}

// mcpCommandIn reads the command the entry of a file names, or "".
func mcpCommandIn(t *testing.T, a autoArtifact) string {
	t.Helper()
	bin, err := autoMCPBinaryIn(a.Path, a.Type)
	if err != nil {
		return ""
	}
	return bin
}

func TestAutoMCPInstallCycle(t *testing.T) {
	for _, tc := range autoMCPCases {
		t.Run(tc.name, func(t *testing.T) {
			autoTestMCPCycle(t, tc.agent, tc.scope)
		})
	}
}

func autoTestMCPCycle(t *testing.T, agent guard.Agent, scope autoScope) {
	t.Helper()
	env := autoNewTestEnv(t)
	env.seedForeign(t)
	env.seedForeignMCP(t)
	entry := env.mcpEntry(t, agent, scope)
	bin, err := guard.Binary()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--agent", string(agent), "--scope", string(scope)}
	if err := env.run(t, append([]string{"agent", "install"}, args...)...); err != nil {
		t.Fatalf("install: %v", err)
	}
	if mcpCommandIn(t, entry) != "" {
		t.Fatal("install without --mcp wrote the server entry")
	}
	if err := env.run(t, append([]string{"agent", "install", "--mcp"}, args...)...); err != nil {
		t.Fatalf("install --mcp: %v", err)
	}
	if got := mcpCommandIn(t, entry); got != bin {
		t.Fatalf("entry names %q, want %q", got, bin)
	}
	if err := env.run(t, append([]string{"--json", "agent", "install", "--mcp"}, args...)...); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	autoExpectAllActions(t, env.report(t), autoActionUnchanged)
	if err := env.run(t, append([]string{"agent", "uninstall", "--mcp"}, args...)...); err != nil {
		t.Fatalf("uninstall --mcp: %v", err)
	}
	autoExpectMCPRemoved(t, entry)
	if agent != guard.AgentClaude && !autoExists(autoSkillPath(env.home, agent)) {
		t.Fatal("uninstall --mcp removed the skill")
	}
	if err := env.run(t, append([]string{"agent", "uninstall"}, args...)...); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	autoExpectRemoved(t, env, agent, scope)
	autoExpectMCPRemoved(t, entry)
}

// autoExpectMCPRemoved checks that only the foreign content remains.
func autoExpectMCPRemoved(t *testing.T, entry autoArtifact) {
	t.Helper()
	if entry.Type == autoTypeTOML {
		data, err := os.ReadFile(entry.Path)
		if err != nil || !bytes.Equal(data, autoForeignCodexConfig) {
			t.Fatalf("%s after uninstall = %q (%v)", entry.Path, data, err)
		}
		return
	}
	if got := autoReadTestJSON(t, entry.Path); !reflect.DeepEqual(got, autoForeignMCPJSON) {
		t.Fatalf("%s after uninstall = %v", entry.Path, got)
	}
}

func TestAutoMCPCodexConflictIsLeftAlone(t *testing.T) {
	env := autoNewTestEnv(t)
	config := filepath.Join(env.home, autoCodexDir, autoCodexConfigFile)
	foreign := []byte("[mcp_servers.gh-board]\ncommand = \"/elsewhere/gh-board\"\nargs = [\"mcp\"]\n")
	autoWriteTestFile(t, config, foreign)
	if err := env.run(t, "--json", "agent", "install", "--agent", "codex", "--mcp"); err != nil {
		t.Fatal(err)
	}
	report := env.report(t)
	var notes []string
	for _, f := range report.Agents[0].Files {
		if f.Path == config {
			notes = f.Notes
		}
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "already defined outside") {
		t.Fatalf("notes = %v", notes)
	}
	if data, _ := os.ReadFile(config); !bytes.Equal(data, foreign) {
		t.Fatalf("config.toml changed: %q", data)
	}
}

func TestAutoMCPUninstallWithoutReceipt(t *testing.T) {
	env := autoNewTestEnv(t)
	env.seedForeignMCP(t)
	if err := env.run(t, "agent", "install", "--agent", "all", "--mcp"); err != nil {
		t.Fatal(err)
	}
	for _, agent := range guard.Agents {
		if err := os.Remove(autoReceiptPath(env.deps.Dirs, agent, autoScopeUser)); err != nil {
			t.Fatal(err)
		}
	}
	if err := env.run(t, "agent", "uninstall", "--agent", "all"); err != nil {
		t.Fatal(err)
	}
	for _, agent := range guard.Agents {
		autoExpectMCPRemoved(t, env.mcpEntry(t, agent, autoScopeUser))
	}
}

func TestAutoMCPInstallWritesStdioEntry(t *testing.T) {
	env := autoNewTestEnv(t)
	if err := env.run(t, "agent", "install", "--agent", "claude", "--mcp"); err != nil {
		t.Fatal(err)
	}
	obj := autoReadTestJSON(t, filepath.Join(env.home, autoClaudeUserMCP))
	entry := obj["mcpServers"].(map[string]any)["gh-board"].(map[string]any)
	if entry["type"] != "stdio" || !reflect.DeepEqual(entry["args"], []any{"mcp"}) || entry["command"] == "" {
		t.Fatalf("entry = %v", entry)
	}
	toml := string(autoMCPTOML(`C:\Program Files\gh "board"\gh-board.exe`))
	if !strings.Contains(toml, `command = "C:\\Program Files\\gh \"board\"\\gh-board.exe"`) {
		t.Fatalf("toml = %s", toml)
	}
	block := autoUpsertBlockWith(nil, []byte(toml), autoTOMLMarkers)
	if bin, ok := autoTOMLCommand(block); !ok || bin != `C:\Program Files\gh "board"\gh-board.exe` {
		t.Fatalf("command = %q %v", bin, ok)
	}
	if _, ok := autoTOMLCommand([]byte("# gh-board:begin\ncommand = unquoted\n# gh-board:end\n")); ok {
		t.Fatal("unquoted command accepted")
	}
	if _, ok := autoTOMLCommand([]byte("# gh-board:begin\nargs = []\n# gh-board:end\n")); ok {
		t.Fatal("block without command accepted")
	}
}
