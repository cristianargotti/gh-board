package guard_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

var liveShells = [][]string{
	{"sh", "-c"},
	{"bash", "-lc"},
	{"/bin/bash", "-lc"},
	{"ZSH.EXE", "-C"},
	{"/bin/zsh.exe", "-f", "-ic"},
	{"pwsh", "-c"},
	{"pwsh.exe", "-NoProfile", "-Command"},
	{"powershell", "-Command"},
	{"powershell.exe", "-Command"},
	{"powershell.exe", "-c"},
	{"powershell", "-NoProfile", "-Command"},
	{"powershell", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-NoProfile", "-cOmMaNd"},
	{"pwsh.exe", "-Command", "-NoProfile"},
	{"powershell", "-c", "-NoLogo", "-ExecutionPolicy", "Bypass", "-NoProfile"},
	{`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, "-NoProfile", "-Command"},
	{`C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe`, "-NoProfile", "-Command"},
	{`C:\Program Files\PowerShell\7\PWSH.EXE`, "-NoProfile", "-C"},
	{"cmd", "/c"},
	{"cmd.exe", "/c"},
	{"CMD.EXE", "/D", "/S", "/C"},
	{`C:\Windows\System32\cmd.exe`, "/c"},
}

var liveScripts = []struct {
	command string
	deny    bool
}{
	{"gh project delete 999999 --owner nobody", true},
	{"gh api repos/x/y --method=delete", true},
	{"gh.exe project delete 1", true},
	{"gh project list", false},
	{"git status", false},
	{"GH_DEBUG=api gh api rate_limit", false},
	{`gh api graphql -f query='query { viewer { login } }'`, false},
	{`echo 'gh project delete 1'`, false},
}

func TestLiveWrapperArgv(t *testing.T) {
	for _, shell := range liveShells {
		for _, script := range liveScripts {
			argv := append(append([]string(nil), shell...), script.command)
			for _, strict := range []bool{false, true} {
				checkLiveInput(t, guard.AgentCodex, "Bash", argv, strict, strict || script.deny)
			}
		}
	}
}

func TestLiveWrapperStrings(t *testing.T) {
	for _, shell := range liveShells {
		for _, script := range liveScripts {
			command := shellQuote(shell[0]) + " " + strings.Join(shell[1:], " ") + " " + shellQuote(script.command)
			c := matrixCase{Command: command, Normal: "allow", Strict: "deny"}
			if script.deny {
				c.Normal = "deny"
			}
			for _, agent := range guard.Agents {
				checkAgent(t, c, agent, false)
				checkAgent(t, c, agent, true)
			}
		}
	}
}

func shellQuote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func TestLivePowerShellTool(t *testing.T) {
	for _, script := range liveScripts {
		for _, strict := range []bool{false, true} {
			checkLiveInput(t, guard.AgentClaude, "PowerShell", script.command, strict, script.deny)
		}
	}
}

func checkLiveInput(t *testing.T, agent guard.Agent, tool string, command any, strict, want bool) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"tool_name": tool, "tool_input": map[string]any{"command": command}})
	if err != nil {
		t.Fatal(err)
	}
	ans, denied, err := guard.CheckWith(agent, data, guard.CheckOptions{Strict: strict})
	if err != nil || denied != want || (denied && ans.ExitCode != 2) {
		t.Fatalf("%s %s %v strict=%v: denied=%v answer=%+v err=%v", agent, tool, command, strict, denied, ans, err)
	}
}

var liveFlags = []string{"-X ", "-X", "-X=", "--method ", "--method="}

// Generate the case permutations independently of the rule generator.
func TestEveryMethodCase(t *testing.T) {
	for bits := range 64 {
		method := []byte("delete")
		for i := range method {
			if bits&(1<<i) != 0 {
				method[i] -= 'a' - 'A'
			}
		}
		for _, flag := range liveFlags {
			assertMethodSpellings(t, flag+string(method))
		}
	}
}

func assertMethodSpellings(t *testing.T, method string) {
	t.Helper()
	for _, command := range []string{"gh api " + method + " repos/x/y", "gh api repos/x/y " + method} {
		for _, strict := range []bool{false, true} {
			for _, agent := range guard.Agents {
				checkAgent(t, matrixCase{Command: command, Normal: "deny", Strict: "deny"}, agent, strict)
			}
		}
	}
}

func TestNestedShells(t *testing.T) {
	command := `sh -c 'powershell.exe -NoProfile -Command "GH api --method delete repos/x/y"'`
	if _, denied, err := guard.Evaluate(command, false); err != nil || !denied {
		t.Fatalf("nested wrappers: denied=%v err=%v", denied, err)
	}
}

func TestRuleSpellings(t *testing.T) {
	rules, err := guard.ClaudeDenyRules(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"Bash", "PowerShell"} {
		for _, program := range []string{"gh", "gh.exe"} {
			for _, method := range []string{"DELETE", "delete", "Delete", "dElEtE"} {
				for _, flag := range liveFlags {
					assertRulePresent(t, rules, tool, program, flag+method)
				}
			}
		}
	}
}

func assertRulePresent(t *testing.T, rules []string, tool, program, method string) {
	t.Helper()
	for _, pattern := range []string{" api " + method + " *", " api * " + method + "*"} {
		want := tool + "(" + program + pattern + ")"
		if !contains(rules, want) {
			t.Errorf("rules lack %s", want)
		}
	}
}

func BenchmarkGuardCheck(b *testing.B) {
	for b.Loop() {
		_, _, _ = guard.Check(guard.AgentClaude, []byte(claudeDelete))
	}
}

func TestProgramDoesNotFoldSubcommands(t *testing.T) {
	for _, program := range []string{"gh", "GH", "Gh", "gh.exe", "GH.EXE"} {
		command := strings.Join([]string{program, "Project", "delete", "1"}, " ")
		if _, denied, err := guard.Evaluate(command, true); err != nil || denied {
			t.Fatalf("%s denied=%v err=%v", command, denied, err)
		}
	}
}
