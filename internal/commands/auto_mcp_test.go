package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var mcpUpdateGolden = flag.Bool("update-mcp-golden", false, "rewrite the golden file of the MCP catalog")

// mcpInit is the legacy handshake every scripted session opens with.
const mcpInit = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n" +
	`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

// mcpSession runs requests through "gh board mcp" with the deps and
// returns the responses after the handshake, decoded, in order.
func mcpSession(t *testing.T, deps *Deps, globals []string, requests ...string) []map[string]any {
	t.Helper()
	deps.In = strings.NewReader(mcpInit + strings.Join(requests, "\n") + "\n")
	out := &bytes.Buffer{}
	deps.Out = out
	if err := Execute(context.Background(), append(globals, "mcp"), deps); err != nil {
		t.Fatalf("mcp: %v", err)
	}
	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	if len(responses) != len(requests)+1 {
		t.Fatalf("responses = %d, want %d:\n%s", len(responses), len(requests)+1, out.String())
	}
	return responses[1:]
}

// mcpCall builds a tools/call request line.
func mcpCall(t *testing.T, id int, name string, args map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// mcpToolResult reads the result of a tools/call response: the envelope,
// the first text line and the error flag.
func mcpToolResult(t *testing.T, resp map[string]any) (map[string]any, string, bool) {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("not a result: %v", resp)
	}
	content := result["content"].([]any)
	line := content[0].(map[string]any)["text"].(string)
	isError, _ := result["isError"].(bool)
	return result["structuredContent"].(map[string]any), line, isError
}

func TestAutoMCPToolsListGolden(t *testing.T) {
	deps, _ := readTestDeps(t, readNewFakeReader())
	responses := mcpSession(t, deps, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	got, err := json.MarshalIndent(responses[0]["result"], "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "mcp", "tools_list.golden")
	if *mcpUpdateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update-mcp-golden to create it)", err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Fatalf("tools/list differs from the golden file\n--- got ---\n%s", got)
	}
}

// mcpNeverTools are the verbs section 8.4 keeps out of the catalog, as
// tool names and as the first word of a command path.
var mcpNeverTools = []string{"apply", "agent", "guard", "watch", "init", "milestone", "plan", "mcp", "schema", "doctor", "log", "version", "me", "standup"}

func TestAutoMCPCatalogIsExact(t *testing.T) {
	root := NewRoot(&Deps{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	catalog, err := autoMCPCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "context status attention roadmap item list search epics sprint move assign unassign set comment new close tidy digest_post"
	var names []string
	for _, tool := range catalog.tools {
		names = append(names, tool.Name)
		for _, never := range mcpNeverTools {
			if tool.Name == never || catalog.specs[tool.Name].path[0] == never {
				t.Errorf("%s must never be a tool", never)
			}
		}
		if !strings.Contains(tool.InputSchema["properties"].(map[string]any)[mcpProjectProperty].(map[string]any)["type"].(string), "string") {
			t.Errorf("%s lacks the project field", tool.Name)
		}
	}
	if strings.Join(names, " ") != want {
		t.Fatalf("tools = %v", names)
	}
	if spec := catalog.specs["sprint"]; strings.Join(spec.which, ",") != "current,next" {
		t.Fatalf("sprint choices = %v", spec.which)
	}
	if spec := catalog.specs["set"]; len(spec.args) != 2 || !spec.args[1].variadic || spec.args[1].name != "fields" {
		t.Fatalf("set args = %+v", spec.args)
	}
}

func TestAutoMCPReadAndOverride(t *testing.T) {
	deps, _ := readTestDeps(t, readNewFakeReader())
	responses := mcpSession(t, deps, []string{"--project", "acme/7"},
		mcpCall(t, 1, "context", map[string]any{"for": "bruno", "budget": 300}),
		mcpCall(t, 2, "sprint", map[string]any{"which": "next"}),
		mcpCall(t, 3, "attention", map[string]any{"project": "acme/8"}),
		mcpCall(t, 4, "list", map[string]any{"all": true, "status": "IN PROGRESS"}),
	)
	envelope, line, isError := mcpToolResult(t, responses[0])
	data := envelope["data"].(map[string]any)
	if isError || line != "context: ok" || data["viewer"] != "ana" || data["completeness"] == nil {
		t.Fatalf("context = %v %q %v", envelope, line, isError)
	}
	if envelope, _, isError := mcpToolResult(t, responses[1]); isError || envelope["data"].(map[string]any)["which"] != "next" {
		t.Fatalf("sprint next = %v", envelope)
	}
	envelope, line, isError = mcpToolResult(t, responses[2])
	if !isError || envelope["exit_code"] != float64(domain.ExitNotFound) || !strings.Contains(line, "exit code 2") {
		t.Fatalf("attention on acme/8 = %v %q", envelope, line)
	}
	if envelope, _, isError := mcpToolResult(t, responses[3]); isError || envelope["data"].(map[string]any)["shown"] != float64(2) {
		t.Fatalf("list = %v", envelope)
	}
}

func TestAutoMCPWriteDryRunThenReal(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	responses := mcpSession(t, deps, nil,
		mcpCall(t, 1, "comment", map[string]any{"ref": "1", "text": "Olá, time.", "dry_run": true, "reason": "teste"}),
		mcpCall(t, 2, "comment", map[string]any{"ref": "1", "text": "Olá, time.", "reason": "teste"}),
		mcpCall(t, 3, "comment", map[string]any{"ref": "1", "text": "-"}),
	)
	envelope, _, isError := mcpToolResult(t, responses[0])
	if isError || envelope["data"].(map[string]any)["dry_run"] != true {
		t.Fatalf("dry run = %v", envelope)
	}
	envelope, line, isError := mcpToolResult(t, responses[1])
	if isError || line != "comment: ok" || envelope["data"].(map[string]any)["dry_run"] != false {
		t.Fatalf("real = %v %q", envelope, line)
	}
	// Three calls, one of them real: the fake writer saw exactly one comment.
	if comments := strings.Count(strings.Join(fake.calls, " "), "comment"); comments != 1 {
		t.Fatalf("comment calls = %d: %v", comments, fake.calls)
	}
	if _, line, isError := mcpToolResult(t, responses[2]); !isError || !strings.Contains(line, "comment must not be empty") {
		t.Fatalf("stdin body must be empty under MCP: %q", line)
	}
}

func TestAutoMCPPolicyAndDriftCodes(t *testing.T) {
	deps, _, _ := writeFixture(t)
	deps.Config.Config.Policy.Transitions["Ready"] = []string{"Done"}
	responses := mcpSession(t, deps, nil,
		mcpCall(t, 1, "move", map[string]any{"ref": "1", "status": "Active"}),
		mcpCall(t, 2, "comment", map[string]any{"ref": "1", "text": "x", "expect": []any{"Flow=Done"}}),
		mcpCall(t, 3, "set", map[string]any{"ref": "1", "fields": []any{"Lane=Platform"}}),
	)
	envelope, line, isError := mcpToolResult(t, responses[0])
	if !isError || envelope["exit_code"] != float64(domain.ExitPolicy) || !strings.Contains(line, "exit code 3, policy") {
		t.Fatalf("policy = %v %q", envelope, line)
	}
	envelope, line, isError = mcpToolResult(t, responses[1])
	if !isError || envelope["exit_code"] != float64(domain.ExitDrift) || !strings.Contains(line, "exit code 4, drift") {
		t.Fatalf("drift = %v %q", envelope, line)
	}
	if _, line, isError := mcpToolResult(t, responses[2]); isError {
		t.Fatalf("set = %q", line)
	}
}

func TestAutoMCPPlanToolReturnsApplyLine(t *testing.T) {
	deps, _, writer, _ := planTestDeps(t)
	responses := mcpSession(t, deps, nil,
		mcpCall(t, 1, "close", map[string]any{"ref": "#7", "dry_run": true}),
		mcpCall(t, 2, "close", map[string]any{"ref": "#7"}),
		mcpCall(t, 3, "digest_post", map[string]any{"status": "late"}),
	)
	envelope, _, isError := mcpToolResult(t, responses[0])
	if isError || envelope["dry_run"] != true || envelope["apply"] != nil {
		t.Fatalf("dry run = %v", envelope)
	}
	envelope, line, isError := mcpToolResult(t, responses[1])
	saved := planTestSaved(t, deps)
	if isError || envelope["plan_id"] != saved.ID || envelope["apply"] != "gh board apply "+saved.ID || !strings.Contains(line, "gh board apply "+saved.ID) {
		t.Fatalf("plan = %v %q", envelope, line)
	}
	if len(writer.calls) != 0 {
		t.Fatalf("plan tool wrote: %v", writer.calls)
	}
	if _, line, isError := mcpToolResult(t, responses[2]); !isError || !strings.Contains(line, "exit code 1") {
		t.Fatalf("digest_post with a bad status = %q", line)
	}
	if _, err := plan.List(deps.Dirs.State); err != nil {
		t.Fatal(err)
	}
}

func TestAutoMCPUnknownToolIsProtocolError(t *testing.T) {
	deps, _ := readTestDeps(t, readNewFakeReader())
	responses := mcpSession(t, deps, nil, mcpCall(t, 1, "apply", map[string]any{"id": "x"}))
	errObj, ok := responses[0]["error"].(map[string]any)
	if !ok || errObj["code"] != float64(-32602) || !strings.Contains(errObj["message"].(string), "Unknown tool: apply") {
		t.Fatalf("response = %v", responses[0])
	}
}
