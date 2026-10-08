package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	destructive = "gh project delete 999999 --owner nobody"
	readOnly    = "gh project list"
	claudeAgent = "claude"
	cursorAgent = "cursor"
	codexAgent  = "codex"
	deny        = "deny"
	allow       = "allow"
)

var guardCases = []struct {
	agent string
	tool  string
	code  int
}{
	{claudeAgent, "Bash", 2},
	{cursorAgent, "", 0},
	{codexAgent, "exec_command", 2},
}

func (s *suite) checkGuards() {
	for _, agent := range guardCases {
		for _, text := range []string{destructive, readOnly} {
			want, code := allow, 0
			if text == destructive {
				want, code = deny, agent.code
			}
			input := payload(agent.agent, agent.tool, text)
			r := s.board(input, "guard", "check", "--agent", agent.agent)
			err := guardResult(r, agent.agent, want, code)
			s.record(agent.agent+" "+want, err, fmt.Sprintf("%s, exit %d", want, code))
		}
	}
}

func payload(agent, tool, text string) string {
	quoted, _ := json.Marshal(text)
	if agent == cursorAgent {
		return `{"command":` + string(quoted) + `}`
	}
	return `{"tool_name":"` + tool + `","tool_input":{"command":` + string(quoted) + `}}`
}

func guardResult(r result, agent, want string, code int) error {
	if r.err != nil {
		return r.err
	}
	if r.code != code {
		return fmt.Errorf("exit %d, want %d: %s", r.code, code, r.stderr)
	}
	if want == allow && agent != cursorAgent && strings.TrimSpace(r.stdout) == "" {
		return nil
	}
	var answer struct {
		Permission string `json:"permission"`
		Hook       struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &answer); err != nil {
		return fmt.Errorf("invalid guard JSON: %w", err)
	}
	got := answer.Permission
	if agent != cursorAgent {
		got = answer.Hook.Decision
		if answer.Hook.Event != "PreToolUse" {
			return fmt.Errorf("hook event %q, want PreToolUse", answer.Hook.Event)
		}
	}
	if got != want {
		return fmt.Errorf("decision %q, want %q", got, want)
	}
	return nil
}
