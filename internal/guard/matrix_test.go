package guard_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

// matrixCase is one row of testdata/matrix.json: the outcome of a command
// in normal and strict mode and the rule expected to match first.
type matrixCase struct {
	Command    string `json:"command"`
	Normal     string `json:"normal"`
	Strict     string `json:"strict"`
	Rule       string `json:"rule"`
	StrictRule string `json:"strict_rule"`
	Note       string `json:"note"`
}

func loadMatrix(t *testing.T) []matrixCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []matrixCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.Cases
}

func TestMatrixEvaluate(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range loadMatrix(t) {
		checkMode(t, c, false, covered)
		checkMode(t, c, true, covered)
	}
	rules, err := guard.Rules(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		if !covered[r] {
			t.Errorf("rule %q has no deny case in testdata/matrix.json", r)
		}
	}
}

func checkMode(t *testing.T, c matrixCase, strict bool, covered map[string]bool) {
	t.Helper()
	want, wantRule := c.Normal, c.Rule
	if strict {
		want = c.Strict
		if c.StrictRule != "" {
			wantRule = c.StrictRule
		}
	}
	m, denied, err := guard.Evaluate(c.Command, strict)
	if err != nil {
		t.Fatal(err)
	}
	if denied != (want == "deny") {
		t.Errorf("%q strict=%v: denied=%v (rule %q), want %s. %s", c.Command, strict, denied, m.Rule, want, c.Note)
		return
	}
	if denied {
		covered[m.Rule] = true
		if wantRule != "" && m.Rule != wantRule {
			t.Errorf("%q strict=%v: rule %q, want %q", c.Command, strict, m.Rule, wantRule)
		}
	}
}

func TestMatrixAgents(t *testing.T) {
	for _, c := range loadMatrix(t) {
		for _, agent := range guard.Agents {
			checkAgent(t, c, agent, false)
			checkAgent(t, c, agent, true)
		}
	}
}

func checkAgent(t *testing.T, c matrixCase, agent guard.Agent, strict bool) {
	t.Helper()
	want := c.Normal
	if strict {
		want = c.Strict
	}
	ans, denied, err := guard.CheckWith(agent, agentInput(t, agent, c.Command), guard.CheckOptions{Strict: strict})
	if err != nil {
		t.Fatalf("%s %q: %v", agent, c.Command, err)
	}
	if denied != (want == "deny") {
		t.Errorf("%s %q strict=%v: denied=%v, want %s", agent, c.Command, strict, denied, want)
		return
	}
	wantExit, marker := 0, `"permission":"allow"`
	if denied {
		marker = `"permissionDecision":"deny"`
		if agent == guard.AgentCursor {
			marker = `"permission":"deny"`
		} else {
			wantExit = 2
		}
	} else if agent != guard.AgentCursor {
		marker = ""
	}
	if ans.ExitCode != wantExit || !strings.Contains(string(ans.Body), marker) {
		t.Errorf("%s %q: exit %d body %s, want exit %d with %s", agent, c.Command, ans.ExitCode, ans.Body, wantExit, marker)
	}
}

func agentInput(t *testing.T, agent guard.Agent, command string) []byte {
	t.Helper()
	var in any
	if agent == guard.AgentCursor {
		in = map[string]any{"command": command, "cwd": "/work", "hook_event_name": "beforeShellExecution"}
	} else {
		in = map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}, "hook_event_name": "PreToolUse"}
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
