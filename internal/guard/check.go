package guard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// CheckOptions tunes a check. Strict adds the strict family of section 6.5,
// which the hooks request with --strict when installed in strict mode.
type CheckOptions struct {
	Strict bool
}

const (
	exitAllow = 0
	exitBlock = 2
	// hookEvent is the event both Claude Code and Codex name in the answer.
	hookEvent  = "PreToolUse"
	decideDeny = "deny"
)

// hookAnswer is the Claude Code and Codex PreToolUse decision.
type hookAnswer struct {
	HookSpecificOutput hookOutput `json:"hookSpecificOutput"`
	SystemMessage      string     `json:"systemMessage"`
}

type hookOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// cursorAnswer is the Cursor beforeShellExecution decision.
type cursorAnswer struct {
	Permission   string `json:"permission"`
	UserMessage  string `json:"user_message,omitempty"`
	AgentMessage string `json:"agent_message,omitempty"`
}

// shellToolNames are the tool names under which Claude Code and Codex
// report a shell command (Codex matches its shell and exec tools as Bash).
// Any other tool, apply_patch or an MCP tool among them, carries no shell
// command even when its input has a command member, so it is allowed.
var shellToolNames = map[string]bool{"Bash": true, "PowerShell": true, "shell": true, "local_shell": true, "exec_command": true, "shell_command": true}

// toolInput is the PreToolUse input Claude Code and Codex send: the
// command of the Bash tool sits in tool_input.command.
type toolInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command json.RawMessage `json:"command"`
	} `json:"tool_input"`
}

// cursorInput is the beforeShellExecution input: the command is top level.
type cursorInput struct {
	Command string `json:"command"`
}

// Check evaluates the agent's stdin in normal mode; see CheckWith.
func Check(agent Agent, stdin []byte) (Answer, bool, error) {
	return CheckWith(agent, stdin, CheckOptions{})
}

// CheckWith reads the hook input of the agent, evaluates the command and
// returns the answer in the shape the agent documents. The boolean is true
// when the command is denied. Input of a tool that is not a shell, or
// without a command, is allowed: the Codex hook may run for every tool
// call. Malformed input is a usage error, which Cursor turns into a
// denial through failClosed and the other agents report to the user.
func CheckWith(agent Agent, stdin []byte, opts CheckOptions) (Answer, bool, error) {
	command, ok, err := commandOf(agent, stdin)
	if err != nil {
		return Answer{}, false, err
	}
	if !ok {
		return allowAnswer(agent), false, nil
	}
	m, denied, err := Evaluate(command, opts.Strict)
	if err != nil {
		return Answer{}, false, err
	}
	if !denied {
		return allowAnswer(agent), false, nil
	}
	ans, err := denyAnswer(agent, m)
	return ans, true, err
}

func commandOf(agent Agent, stdin []byte) (string, bool, error) {
	switch agent {
	case AgentCursor:
		var in cursorInput
		if err := decode(agent, stdin, &in); err != nil {
			return "", false, err
		}
		return in.Command, in.Command != "", nil
	case AgentClaude, AgentCodex:
		var in toolInput
		if err := decode(agent, stdin, &in); err != nil {
			return "", false, err
		}
		if in.ToolName != "" && !shellToolNames[in.ToolName] {
			return "", false, nil
		}
		return commandField(agent, in.ToolInput.Command)
	default:
		return "", false, fmt.Errorf("guard check: unknown agent %q: %w", agent, domain.ErrUsage)
	}
}

func decode(agent Agent, stdin []byte, v any) error {
	if err := json.Unmarshal(stdin, v); err != nil {
		return fmt.Errorf("guard check: %s input is not the documented JSON: %v: %w", agent, err, domain.ErrUsage)
	}
	return nil
}

// commandField accepts the documented string and, defensively, an argv
// list; a missing field means the call is not a shell command.
func commandField(agent Agent, raw json.RawMessage) (string, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, text != "", nil
	}
	var argv []string
	if err := json.Unmarshal(raw, &argv); err != nil {
		return "", false, fmt.Errorf("guard check: %s tool_input.command is neither a string nor a list: %w", agent, domain.ErrUsage)
	}
	text = argvCommand(argv)
	return text, text != "", nil
}

// Keeping the wrapper lets strict mode judge it before the inner script.
func argvCommand(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, a := range argv {
		quoted = append(quoted, quoteWord(a))
	}
	return strings.Join(quoted, " ")
}

func quoteWord(w string) string {
	if w != "" && !strings.ContainsAny(w, " \t\n\r'\"\\$`;&|(){}<>") {
		return w
	}
	return "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
}

func allowAnswer(agent Agent) Answer {
	if agent == AgentCursor {
		return Answer{Body: []byte("{\"permission\":\"allow\"}\n"), ExitCode: exitAllow}
	}
	return Answer{ExitCode: exitAllow}
}

func denyAnswer(agent Agent, m Match) (Answer, error) {
	human, machine := humanMessage(m), agentMessage(m)
	var body any
	code := exitBlock
	if agent == AgentCursor {
		body = cursorAnswer{Permission: decideDeny, UserMessage: human, AgentMessage: machine}
		code = exitAllow
	} else {
		body = hookAnswer{
			HookSpecificOutput: hookOutput{HookEventName: hookEvent, PermissionDecision: decideDeny, PermissionDecisionReason: machine},
			SystemMessage:      human,
		}
	}
	data, err := marshal(body)
	if err != nil {
		return Answer{}, err
	}
	return Answer{Body: data, Message: machine, ExitCode: code}, nil
}

// marshal encodes without HTML escaping so that "<id>" stays readable.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("guard check: encode answer: %w", err)
	}
	return buf.Bytes(), nil
}
