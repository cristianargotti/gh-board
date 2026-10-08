package guard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// hookTimeout is the seconds an agent waits for guard check; the check
// runs in milliseconds, so a hang is a defect and must not stall the agent.
const hookTimeout = 10

// executable resolves the running binary; tests replace it to exercise
// failures.
var executable = os.Executable

// Binary returns the absolute path of the running gh-board binary, with
// symlinks resolved. Hooks name it instead of relying on PATH, and doctor
// compares it with the path the installed hooks carry.
func Binary() (string, error) {
	path, err := executable()
	if err != nil {
		return "", fmt.Errorf("guard: resolve binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("guard: resolve binary %s: %w", path, err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("guard: resolve binary %s: %w", resolved, err)
	}
	return abs, nil
}

// HookCommand protects the resolved binary path from shell parsing;
// Windows uses an encoded PowerShell invocation shared by all installers.
func HookCommand(agent Agent, strict bool) (string, error) {
	bin, err := Binary()
	if err != nil {
		return "", err
	}
	return hookCommand(bin, agent, strict, runtime.GOOS), nil
}

// ClaudeDenyRules separates tools to protect PowerShell without disabling it.
func ClaudeDenyRules(strict bool) ([]string, error) {
	rules, err := Rules(strict)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rules))
	for _, r := range expandedRules(rules) {
		out = append(out, "Bash("+r+")", "PowerShell("+r+")")
	}
	return out, nil
}

type cursorHooks struct {
	Version int                     `json:"version"`
	Hooks   map[string][]cursorHook `json:"hooks"`
}

type cursorHook struct {
	Command    string `json:"command"`
	Timeout    int    `json:"timeout"`
	FailClosed bool   `json:"failClosed"`
}

// CursorHook returns the hooks.json fragment of Cursor: one
// beforeShellExecution hook that fails closed, so a crash or a timeout of
// the guard denies the command instead of letting it through.
func CursorHook(strict bool) ([]byte, error) {
	cmd, err := HookCommand(AgentCursor, strict)
	if err != nil {
		return nil, err
	}
	doc := cursorHooks{Version: 1, Hooks: map[string][]cursorHook{
		"beforeShellExecution": {{Command: cmd, Timeout: hookTimeout, FailClosed: true}},
	}}
	return marshalIndent(doc)
}

type toolHooks struct {
	Hooks map[string][]matcherGroup `json:"hooks"`
}

type matcherGroup struct {
	Matcher string        `json:"matcher"`
	Hooks   []commandHook `json:"hooks"`
}

type commandHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// ClaudeHook also matches PowerShell because Windows may omit Bash entirely.
func ClaudeHook(strict bool) ([]byte, error) {
	return toolHook(AgentClaude, strict)
}

// CodexHook returns the hooks.json fragment of Codex: a PreToolUse command
// hook on Bash, which the person trusts once through /hooks.
func CodexHook(strict bool) ([]byte, error) {
	return toolHook(AgentCodex, strict)
}

// Codex reports shell commands as Bash; anchors exclude MCP lookalikes.
const (
	claudeMatcher = "Bash|PowerShell"
	codexMatcher  = "^Bash$"
)

func toolHook(agent Agent, strict bool) ([]byte, error) {
	cmd, err := HookCommand(agent, strict)
	if err != nil {
		return nil, err
	}
	matcher := claudeMatcher
	if agent == AgentCodex {
		matcher = codexMatcher
	}
	doc := toolHooks{Hooks: map[string][]matcherGroup{
		hookEvent: {{Matcher: matcher, Hooks: []commandHook{{Type: "command", Command: cmd, Timeout: hookTimeout}}}},
	}}
	return marshalIndent(doc)
}

func marshalIndent(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("guard: encode hook: %w", err)
	}
	return buf.Bytes(), nil
}
