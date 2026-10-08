package main

import (
	"encoding/json"
	"errors"
	"testing"
)

var answerCases = []struct {
	name, agent, want string
	r                 result
	code              int
	valid             bool
}{
	{"execution error", claudeAgent, deny, result{err: errors.New("start failed")}, 2, false},
	{"wrong exit", claudeAgent, deny, result{code: 0}, 2, false},
	{"bad JSON", claudeAgent, deny, result{code: 2, stdout: "invalid"}, 2, false},
	{"missing decision", claudeAgent, deny, result{code: 2, stdout: `{}`}, 2, false},
	{"wrong event", claudeAgent, deny, result{code: 2, stdout: `{"hookSpecificOutput":{"hookEventName":"PostToolUse","permissionDecision":"deny"}}`}, 2, false},
	{"wrong decision", cursorAgent, deny, result{stdout: `{"permission":"allow"}`}, 0, false},
	{"cursor deny", cursorAgent, deny, result{stdout: `{"permission":"deny"}`}, 0, true},
	{"cursor allow", cursorAgent, allow, result{stdout: `{"permission":"allow"}`}, 0, true},
	{"cursor empty", cursorAgent, allow, result{}, 0, false},
	{"claude allow", claudeAgent, allow, result{}, 0, true},
	{"codex allow", codexAgent, allow, result{stdout: "\n"}, 0, true},
	{"claude explicit allow", claudeAgent, allow, result{stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`}, 0, true},
	{"codex deny", codexAgent, deny, result{code: 2, stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny"}}`}, 2, true},
}

func TestGuardResult(t *testing.T) {
	for _, tc := range answerCases {
		t.Run(tc.name, func(t *testing.T) {
			err := guardResult(tc.r, tc.agent, tc.want, tc.code)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
}

func TestPayload(t *testing.T) {
	for _, agent := range guardCases {
		var value map[string]any
		if err := json.Unmarshal([]byte(payload(agent.agent, agent.tool, destructive)), &value); err != nil {
			t.Fatal(err)
		}
		if agent.agent == cursorAgent {
			if value["command"] != destructive || len(value) != 1 {
				t.Fatal(value)
			}
		} else if value["tool_name"] != agent.tool || value["tool_input"].(map[string]any)["command"] != destructive {
			t.Fatal(value)
		}
	}
}

func TestGuardsContinueAfterFailure(t *testing.T) {
	s := suite{execute: func(command) result { return result{err: errors.New("broken guard")} }}
	s.checkGuards()
	if len(s.rows) != 6 {
		t.Fatal(s.rows)
	}
	for _, r := range s.rows {
		if r.status != fail {
			t.Fatal(r)
		}
	}
}
