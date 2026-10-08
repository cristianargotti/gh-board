package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var updateGolden = flag.Bool("update-mcp-golden", false, "rewrite the golden files of the protocol tests")

// testCatalog is a two tool catalog: one read, one plan.
func testCatalog() []Tool {
	return []Tool{
		{Name: "status", Title: "gh board status", Description: "Board overview.", Tier: TierRead, InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"project": map[string]any{"type": "string", "description": "owner/number"}},
		}},
		{Name: "close", Title: "gh board close", Description: "Plan closing an issue.", Tier: TierPlan, InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"ref":     map[string]any{"type": "string", "description": "issue reference"},
				"dry_run": map[string]any{"type": "boolean", "description": "show only"},
				"budget":  map[string]any{"type": "integer", "description": "tokens"},
				"labels":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"kind":    map[string]any{"type": "string", "enum": []string{"task", "epic"}},
			},
			"required": []string{"ref"},
		}},
	}
}

// testRunner answers by tool name: status succeeds with a document, close
// saves a plan, and the other names map to the exit codes under test.
func testRunner(calls *[]map[string]any) Runner {
	return func(_ context.Context, tool Tool, args map[string]any) Outcome {
		*calls = append(*calls, args)
		switch tool.Name {
		case "status":
			return Outcome{Code: domain.ExitOK, Output: []byte(`{"items":3,"project":"acme/7"}` + "\n"), Stderr: "Warning: shadowed\n"}
		case "close":
			if dry, _ := args["dry_run"].(bool); dry {
				return Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"plan":{"id":"20261008T120000Z-ab12"},"dry_run":true}` + "\n"), Message: "dry run: no plan saved"}
			}
			return Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"plan":{"id":"20261008T120000Z-ab12"},"dry_run":false,"path":"/state/plans/x.json"}` + "\n"), Message: "plan requires human apply: 20261008T120000Z-ab12"}
		}
		return Outcome{Code: domain.ExitUsage, Message: "unexpected tool"}
	}
}

func newTestServer(calls *[]map[string]any) *Server {
	return &Server{Name: "gh-board", Version: "1.2.3", Instructions: "Start with context.", Tools: testCatalog(), Run: testRunner(calls)}
}

// serve runs a script of request lines through a server and returns the
// output.
func serve(t *testing.T, s *Server, script string) string {
	t.Helper()
	out := &bytes.Buffer{}
	if err := s.Serve(context.Background(), strings.NewReader(script), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	return out.String()
}

func checkGolden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update-mcp-golden to create it)", err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if got != string(want) {
		t.Fatalf("%s differs from the golden file\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

const legacyScript = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"ping"}
{"jsonrpc":"2.0","id":3,"method":"tools/list"}
{"jsonrpc":"2.0","id":"4","method":"tools/call","params":{"name":"status","arguments":{"project":"acme/7"}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"close","arguments":{"ref":"acme/app#3"}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"close","arguments":{"ref":"acme/app#3","dry_run":true}}}
{"jsonrpc":"2.0","id":7,"method":"prompts/list"}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"nope"}}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"close","arguments":{"ref":5,"extra":true,"kind":"bug"}}}
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"close","arguments":{"budget":1.5,"labels":[1]}}}
{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"close","arguments":[]}}
{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{}}
{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":12}}
`

func TestHandshakeLegacyGolden(t *testing.T) {
	var calls []map[string]any
	got := serve(t, newTestServer(&calls), legacyScript)
	checkGolden(t, "legacy", got)
	if len(calls) != 3 || calls[0]["project"] != "acme/7" || calls[2]["dry_run"] != true {
		t.Fatalf("calls = %v", calls)
	}
}

const modernScript = `{"jsonrpc":"2.0","id":"d1","method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"0"}}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"status","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}
{"jsonrpc":"2.0","id":4,"method":"ping","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}
{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01"}}}
{"jsonrpc":"2.0","id":6,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":7}}}
`

func TestHandshakeModernGolden(t *testing.T) {
	var calls []map[string]any
	checkGolden(t, "modern", serve(t, newTestServer(&calls), modernScript))
}

const malformedScript = `not json
{"jsonrpc":"2.0","id":1,"method":"tools/list"}
{"jsonrpc":"1.0","id":2,"method":"ping"}
{"jsonrpc":"2.0","id":3}
[{"jsonrpc":"2.0","id":4,"method":"ping"}]
{"jsonrpc":"2.0","method":"nothing/known"}

{"jsonrpc":"2.0","id":5,"method":"ping"}
{"jsonrpc":"2.0","id":6,"method":"initialize","params":{"protocolVersion":"1.0.0"}}
{"jsonrpc":"2.0","id":7,"method":"initialize","params":[]}
{"jsonrpc":"2.0","id":8,"method":"tools/list"}`

func TestMalformedAndOrderGolden(t *testing.T) {
	var calls []map[string]any
	checkGolden(t, "malformed", serve(t, newTestServer(&calls), malformedScript))
}

func TestServeStopsCleanlyAtEOF(t *testing.T) {
	var calls []map[string]any
	s := newTestServer(&calls)
	out := &bytes.Buffer{}
	if err := s.Serve(context.Background(), strings.NewReader(""), out); err != nil || out.Len() != 0 {
		t.Fatalf("empty input: err=%v out=%q", err, out.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Serve(ctx, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), out); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
}

func TestServeRejectsOversizedLine(t *testing.T) {
	var calls []map[string]any
	s := newTestServer(&calls)
	long := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"pad":"` + strings.Repeat("x", maxLine+10) + `"}}` + "\n"
	out := serve(t, s, long+`{"jsonrpc":"2.0","id":2,"method":"ping"}`+"\n")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"code":-32700`) || !strings.Contains(lines[1], `"id":2`) {
		t.Fatalf("output = %s", out)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestServeReportsStreamErrors(t *testing.T) {
	var calls []map[string]any
	s := newTestServer(&calls)
	if err := s.Serve(context.Background(), failingReader{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "broken pipe") {
		t.Fatalf("read error: %v", err)
	}
	if err := s.Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), failingWriter{}); err == nil {
		t.Fatal("write error must be reported")
	}
}

func TestErrorResponseWrapsPlainErrors(t *testing.T) {
	resp := errorResponse(nil, errors.New("boom"))
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"id":null`) || !strings.Contains(string(data), `"code":-32603`) {
		t.Fatalf("response = %s", data)
	}
	if (&rpcError{Code: 1, Message: "m"}).Error() != "jsonrpc 1: m" {
		t.Fatal("rpcError.Error")
	}
}

func TestCallWithoutRunner(t *testing.T) {
	s := &Server{Name: "gh-board", Tools: testCatalog()}
	out := serve(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n"+
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"status"}}`+"\n")
	if !strings.Contains(out, "no runner configured") || !strings.Contains(out, `"isError":true`) {
		t.Fatalf("output = %s", out)
	}
}
