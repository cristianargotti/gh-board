package commands

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoCheckCall records what the guard check command handed to the seam.
type autoCheckCall struct {
	agent guard.Agent
	stdin []byte
	opts  guard.CheckOptions
}

// autoRunCheck runs guard check with a payload on stdin.
func autoRunCheck(t *testing.T, payload string, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	out := &bytes.Buffer{}
	deps := &Deps{Out: out, Err: &bytes.Buffer{}}
	root := NewRoot(deps)
	root.SetIn(strings.NewReader(payload))
	root.SetArgs(append([]string{"guard", "check"}, args...))
	return out, root.ExecuteContext(context.Background())
}

var autoCheckCases = []struct {
	name   string
	answer guard.Answer
	denied bool
	err    error
	code   domain.ExitCode
	errMsg string
	body   string
}{
	{"claude deny", guard.Answer{Body: []byte("{\"d\":1}\n"), Message: "denied by rule", ExitCode: 2}, true, nil, 2, "denied by rule", "{\"d\":1}\n"},
	{"claude deny without message", guard.Answer{Body: []byte("blocked\n"), ExitCode: 2}, true, nil, 2, "blocked", "blocked\n"},
	{"deny bare", guard.Answer{ExitCode: 2}, true, nil, 2, "command denied by the gh-board guard", ""},
	{"cursor deny", guard.Answer{Body: []byte("{\"permission\":\"deny\"}\n"), ExitCode: 0}, true, nil, 0, "", "{\"permission\":\"deny\"}\n"},
	{"allow", guard.Answer{}, false, nil, 0, "", ""},
	{"cannot evaluate", guard.Answer{ExitCode: 2, Message: "bad input"}, false, nil, 2, "could not evaluate the command: bad input", ""},
	{"cannot evaluate bare", guard.Answer{ExitCode: 1}, false, nil, 1, "could not evaluate the command", ""},
	{"check error", guard.Answer{}, false, errors.New("boom"), 1, "boom", ""},
}

func TestAutoGuardCheck(t *testing.T) {
	for _, tc := range autoCheckCases {
		t.Run(tc.name, func(t *testing.T) {
			var call autoCheckCall
			autoGuardCheck = func(agent guard.Agent, stdin []byte, opts guard.CheckOptions) (guard.Answer, bool, error) {
				call = autoCheckCall{agent, stdin, opts}
				return tc.answer, tc.denied, tc.err
			}
			t.Cleanup(func() { autoGuardCheck = guard.CheckWith })
			out, err := autoRunCheck(t, `{"command":"x"}`, "--agent", "claude", "--strict")
			if domain.CodeOf(err) != tc.code || out.String() != tc.body {
				t.Fatalf("err = %v, out = %q", err, out.String())
			}
			if err != nil && !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("err = %q, want %q", err, tc.errMsg)
			}
			if call.agent != guard.AgentClaude || string(call.stdin) != `{"command":"x"}` || !call.opts.Strict {
				t.Fatalf("call = %+v", call)
			}
		})
	}
}

func TestAutoGuardCheckUsage(t *testing.T) {
	if _, err := autoRunCheck(t, "{}"); domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("missing --agent: %v", err)
	}
	if _, err := autoRunCheck(t, "{}", "--agent", "copilot"); domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("unknown agent: %v", err)
	}
}

// TestAutoGuardCheckReal runs the real evaluator through the command.
func TestAutoGuardCheckReal(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"gh project delete 999999 --owner nobody"}}`
	out, err := autoRunCheck(t, payload, "--agent", "claude")
	if domain.CodeOf(err) != 2 || !strings.Contains(out.String(), "deny") {
		t.Fatalf("real deny: err = %v, out = %s", err, out.String())
	}
	out, err = autoRunCheck(t, `{"command":"gh project list"}`, "--agent", "cursor")
	if err != nil || !strings.Contains(out.String(), "allow") {
		t.Fatalf("real allow: err = %v, out = %s", err, out.String())
	}
}
