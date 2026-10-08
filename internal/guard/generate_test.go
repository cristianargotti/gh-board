package guard_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

func TestClaudeDenyRules(t *testing.T) {
	normal, err := guard.ClaudeDenyRules(false)
	if err != nil {
		t.Fatal(err)
	}
	strict, err := guard.ClaudeDenyRules(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(normal) != 2824 || len(strict) != 2930 {
		t.Fatalf("normal %d strict %d", len(normal), len(strict))
	}
	for _, r := range strict {
		if (!strings.HasPrefix(r, "Bash(") && !strings.HasPrefix(r, "PowerShell(")) || !strings.HasSuffix(r, ")") {
			t.Errorf("rule %q is not in Bash( ) or PowerShell( ) form", r)
		}
	}
	if strings.Join(normal, "\n") != strings.Join(strict[:len(normal)], "\n") {
		t.Fatal("the strict set must extend the normal set")
	}
	if !contains(strict, "Bash(sh -c *)") || contains(normal, "Bash(sh -c *)") {
		t.Fatal("sh -c belongs to the strict set only")
	}
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func TestBinary(t *testing.T) {
	bin, err := guard.Binary()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(bin) {
		t.Fatalf("binary %q is not absolute", bin)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	if want, _ := filepath.EvalSymlinks(exe); want != bin {
		t.Fatalf("binary %q, want %q", bin, want)
	}
}

func TestHookCommand(t *testing.T) {
	bin, _ := guard.Binary()
	got, err := guard.HookCommand(guard.AgentCursor, false)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(got, "powershell.exe ") {
			t.Fatalf("Windows hook: %s", got)
		}
		return
	}
	if want := `"` + bin + `" guard check --agent cursor`; got != want {
		t.Fatalf("command %q, want %q", got, want)
	}
	strict, _ := guard.HookCommand(guard.AgentCodex, true)
	if strict != `"`+bin+`" guard check --agent codex --strict` {
		t.Fatalf("strict command %q", strict)
	}
}

type cursorHooksDoc struct {
	Version int `json:"version"`
	Hooks   map[string][]struct {
		Command    string `json:"command"`
		Timeout    int    `json:"timeout"`
		FailClosed bool   `json:"failClosed"`
	} `json:"hooks"`
}

func TestCursorHook(t *testing.T) {
	for _, strict := range []bool{false, true} {
		data, err := guard.CursorHook(strict)
		if err != nil {
			t.Fatal(err)
		}
		var doc cursorHooksDoc
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%v in %s", err, data)
		}
		hooks := doc.Hooks["beforeShellExecution"]
		if doc.Version != 1 || len(hooks) != 1 || !hooks[0].FailClosed || hooks[0].Timeout != 10 {
			t.Fatalf("fragment %s", data)
		}
		if want, _ := guard.HookCommand(guard.AgentCursor, strict); hooks[0].Command != want {
			t.Fatalf("command %q, want %q", hooks[0].Command, want)
		}
	}
}

type toolHooksDoc struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

var toolHookGenerators = map[guard.Agent]func(bool) ([]byte, error){
	guard.AgentClaude: guard.ClaudeHook,
	guard.AgentCodex:  guard.CodexHook,
}

func TestToolHooks(t *testing.T) {
	for agent, gen := range toolHookGenerators {
		for _, strict := range []bool{false, true} {
			data, err := gen(strict)
			if err != nil {
				t.Fatal(err)
			}
			assertToolHook(t, agent, strict, data)
		}
	}
}

func assertToolHook(t *testing.T, agent guard.Agent, strict bool, data []byte) {
	t.Helper()
	var doc toolHooksDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%v in %s", err, data)
	}
	groups := doc.Hooks["PreToolUse"]
	wantMatcher := "Bash|PowerShell"
	if agent == guard.AgentCodex {
		wantMatcher = "^Bash$"
	}
	if len(groups) != 1 || groups[0].Matcher != wantMatcher || len(groups[0].Hooks) != 1 {
		t.Fatalf("%s fragment %s", agent, data)
	}
	matcher := regexp.MustCompile(groups[0].Matcher)
	if !matcher.MatchString("Bash") || (agent == guard.AgentCodex && (matcher.MatchString("mcp__Bash__run") || matcher.MatchString("apply_patch"))) {
		t.Fatalf("%s matcher %q must select the shell tool only", agent, groups[0].Matcher)
	}
	hook := groups[0].Hooks[0]
	want, _ := guard.HookCommand(agent, strict)
	if hook.Type != "command" || hook.Timeout != 10 || hook.Command != want {
		t.Fatalf("%s hook %+v, want command %q", agent, hook, want)
	}
}
