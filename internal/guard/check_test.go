package guard_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

type checkCase struct {
	name   string
	agent  guard.Agent
	stdin  string
	strict bool
	deny   bool
	exit   int
	body   []string
	empty  bool
}

const (
	claudeDelete = `{"tool_name":"Bash","tool_input":{"command":"gh project delete 999999 --owner acme"}}`
	claudeShell  = `{"tool_name":"Bash","tool_input":{"command":"sh -c 'gh project delete 1'"}}`
	denyMarker   = `"permissionDecision":"deny"`
)

var checkCases = []checkCase{
	{name: "claude deny", agent: guard.AgentClaude, stdin: claudeDelete, deny: true, exit: 2, body: []string{
		`"hookEventName":"PreToolUse"`, denyMarker, `rule 'gh project delete *'`, `"systemMessage":"gh board: comando bloqueado`,
	}},
	{name: "claude allow", agent: guard.AgentClaude, stdin: `{"tool_name":"Bash","tool_input":{"command":"gh project list --owner acme"}}`, empty: true},
	{name: "claude other tool", agent: guard.AgentClaude, stdin: `{"tool_name":"Read","tool_input":{"file_path":"/x"}}`, empty: true},
	{name: "claude null command", agent: guard.AgentClaude, stdin: `{"tool_name":"Bash","tool_input":{"command":null}}`, empty: true},
	{name: "claude strict", agent: guard.AgentClaude, stdin: claudeShell, strict: true, deny: true, exit: 2, body: []string{`rule 'sh -c *'`, "strict mode"}},
	{name: "claude strict off", agent: guard.AgentClaude, stdin: claudeShell, deny: true, exit: 2},
	{name: "codex deny", agent: guard.AgentCodex, stdin: claudeDelete, deny: true, exit: 2, body: []string{denyMarker}},
	{name: "codex argv shell", agent: guard.AgentCodex, stdin: `{"tool_name":"Bash","tool_input":{"command":["bash","-lc","gh project delete 999999"]}}`, deny: true, exit: 2, body: []string{denyMarker}},
	{name: "codex argv list", agent: guard.AgentCodex, stdin: `{"tool_name":"Bash","tool_input":{"command":["gh","issue","delete","1","--repo","acme/x"]}}`, deny: true, exit: 2, body: []string{`rule 'gh issue delete *'`}},
	{name: "codex argv quoted word", agent: guard.AgentCodex, stdin: `{"tool_name":"Bash","tool_input":{"command":["echo","gh project delete 1","it's"]}}`, empty: true},
	{name: "codex argv shell path", agent: guard.AgentCodex, stdin: `{"tool_name":"Bash","tool_input":{"command":["/bin/zsh","-ic","gh repo delete acme/x"]}}`, deny: true, exit: 2},
	{name: "codex apply_patch carries no shell", agent: guard.AgentCodex, stdin: `{"tool_name":"apply_patch","tool_input":{"command":"*** Begin Patch\n+gh project delete 999999 --owner nobody\n*** End Patch"}}`, empty: true},
	{name: "codex mcp tool", agent: guard.AgentCodex, stdin: `{"tool_name":"mcp__filesystem__read_file","tool_input":{"command":"gh repo delete acme/x"}}`, empty: true},
	{name: "codex shell tool name", agent: guard.AgentCodex, stdin: `{"tool_name":"shell","tool_input":{"command":"gh repo delete acme/x"}}`, deny: true, exit: 2},
	{name: "codex without tool name", agent: guard.AgentCodex, stdin: `{"tool_input":{"command":"gh repo delete acme/x"}}`, deny: true, exit: 2},
	{name: "claude edit tool", agent: guard.AgentClaude, stdin: `{"tool_name":"Edit","tool_input":{"command":"gh repo delete acme/x"}}`, empty: true},
	{name: "cursor deny", agent: guard.AgentCursor, stdin: `{"command":"gh repo delete acme/x --yes","cwd":"/w","hook_event_name":"beforeShellExecution"}`, deny: true, body: []string{
		`"permission":"deny"`, `"user_message":"gh board: comando bloqueado`, `"agent_message":"gh board guard: denied`,
	}},
	{name: "cursor allow", agent: guard.AgentCursor, stdin: `{"command":"gh project list"}`, body: []string{`{"permission":"allow"}`}},
	{name: "cursor empty command", agent: guard.AgentCursor, stdin: `{"command":""}`, body: []string{`{"permission":"allow"}`}},
}

func TestCheckWith(t *testing.T) {
	for _, c := range checkCases {
		t.Run(c.name, func(t *testing.T) {
			ans, denied, err := guard.CheckWith(c.agent, []byte(c.stdin), guard.CheckOptions{Strict: c.strict})
			if err != nil {
				t.Fatal(err)
			}
			assertAnswer(t, c, ans, denied)
		})
	}
}

func assertAnswer(t *testing.T, c checkCase, ans guard.Answer, denied bool) {
	t.Helper()
	if denied != c.deny || ans.ExitCode != c.exit {
		t.Fatalf("denied %v exit %d, want %v and %d", denied, ans.ExitCode, c.deny, c.exit)
	}
	body := string(ans.Body)
	if c.empty && body != "" {
		t.Fatalf("expected no body, got %s", body)
	}
	if !c.empty && !strings.HasSuffix(body, "\n") {
		t.Fatalf("body must end with a newline: %q", body)
	}
	for _, want := range c.body {
		if !strings.Contains(body, want) {
			t.Errorf("body %s\nlacks %s", body, want)
		}
	}
	if denied && ans.Message == "" {
		t.Error("a denial must carry the agent reason for stderr")
	}
}

var checkErrors = []struct {
	name  string
	agent guard.Agent
	stdin string
}{
	{"malformed", guard.AgentClaude, `{"tool_name":`},
	{"empty", guard.AgentCursor, ``},
	{"wrong type", guard.AgentCodex, `{"tool_input":{"command":5}}`},
	{"unknown agent", guard.Agent("copilot"), `{}`},
}

func TestCheckErrors(t *testing.T) {
	for _, c := range checkErrors {
		if _, _, err := guard.Check(c.agent, []byte(c.stdin)); !errors.Is(err, domain.ErrUsage) {
			t.Errorf("%s: got %v, want ErrUsage", c.name, err)
		}
	}
}

func TestCheckDecodedAnswer(t *testing.T) {
	ans, denied, err := guard.Check(guard.AgentClaude, []byte(claudeDelete))
	if err != nil || !denied {
		t.Fatalf("denied %v err %v", denied, err)
	}
	var out struct {
		Hook struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
		System string `json:"systemMessage"`
	}
	if err := json.Unmarshal(ans.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Hook.Event != "PreToolUse" || out.Hook.Decision != "deny" || out.Hook.Reason != ans.Message {
		t.Fatalf("decoded %+v", out)
	}
	if !strings.Contains(string(ans.Body), "runs gh board apply <id> in a terminal") {
		t.Fatalf("reason must keep <id> readable: %s", ans.Body)
	}
	if !strings.Contains(out.System, "gh project delete 999999 --owner acme") {
		t.Fatalf("system message must quote the command: %s", out.System)
	}
}

func TestCheckClipsLongCommands(t *testing.T) {
	long := "gh project delete " + strings.Repeat("9", 300) + "\t--owner\tacme"
	stdin, _ := json.Marshal(map[string]string{"command": long})
	ans, denied, err := guard.Check(guard.AgentCursor, stdin)
	if err != nil || !denied {
		t.Fatalf("denied %v err %v", denied, err)
	}
	if strings.Contains(ans.Message, "\t") || !strings.Contains(ans.Message, "...") {
		t.Fatalf("message must be clipped and free of control characters: %s", ans.Message)
	}
}
