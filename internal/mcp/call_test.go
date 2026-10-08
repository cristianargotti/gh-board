package mcp

import (
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var resultCases = []struct {
	name     string
	out      Outcome
	isError  bool
	line     string
	keys     []string
	notInEnv []string
}{
	{"ok", Outcome{Code: domain.ExitOK, Output: []byte(`{"a":1}`)}, false, "status: ok", []string{"data"}, []string{"message", "apply"}},
	{"ok text", Outcome{Code: domain.ExitOK, Output: []byte("plain\n")}, false, "status: ok", []string{"output"}, []string{"data"}},
	{"policy", Outcome{Code: domain.ExitPolicy, Message: "WIP limit reached", Stderr: "warn\n"}, true, "status: WIP limit reached (exit code 3, policy)", []string{"message", "stderr"}, []string{"data"}},
	{"drift", Outcome{Code: domain.ExitDrift, Message: `Status: expected "Ready", found "Done"`}, true, "(exit code 4, drift)", []string{"message"}, nil},
	{"api without message", Outcome{Code: domain.ExitAPI}, true, "status: api (exit code 7, api)", []string{"message"}, nil},
	{"plan", Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"plan":{"id":"P1"},"dry_run":false}`)}, false, "gh board apply P1", []string{"plan_id", "apply", "data"}, []string{"message"}},
	{"plan dry run", Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"plan":{"id":"P1"},"dry_run":true}`)}, false, "dry run, plan P1 was not saved", []string{"plan_id", "dry_run"}, []string{"apply"}},
	{"plan required without plan", Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"nothing_to_do":true}`), Message: "a plan is required"}, true, "(exit code 5, plan-required)", []string{"message", "data"}, []string{"plan_id"}},
	{"plan without id", Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"plan":{"id":""}}`), Message: "x"}, true, "exit code 5", nil, []string{"plan_id"}},
	{"plan not object", Outcome{Code: domain.ExitPlanRequired, Output: []byte(`{"plan":"x"}`), Message: "x"}, true, "exit code 5", nil, []string{"plan_id"}},
	{"plan array", Outcome{Code: domain.ExitPlanRequired, Output: []byte(`[1]`), Message: "x"}, true, "exit code 5", nil, []string{"plan_id"}},
}

func TestBuildResult(t *testing.T) {
	for index, tc := range resultCases {
		t.Run(tc.name, func(t *testing.T) {
			checkResultCase(t, index)
		})
	}
}

func checkResultCase(t *testing.T, index int) {
	t.Helper()
	tc := resultCases[index]
	result := buildResult("status", tc.out)
	if result["isError"] != tc.isError {
		t.Fatalf("isError = %v", result["isError"])
	}
	content := result["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), tc.line) {
		t.Fatalf("line = %q, want %q", content[0]["text"], tc.line)
	}
	envelope := result["structuredContent"].(map[string]any)
	for _, key := range tc.keys {
		if _, ok := envelope[key]; !ok {
			t.Errorf("envelope lacks %q: %v", key, envelope)
		}
	}
	for _, key := range tc.notInEnv {
		if _, ok := envelope[key]; ok {
			t.Errorf("envelope has %q: %v", key, envelope)
		}
	}
	if envelope["exit_code"] != int(tc.out.Code) || envelope["status"] != tc.out.Code.String() {
		t.Errorf("envelope = %v", envelope)
	}
}

func TestStructuredOutputIsRepeatedAsText(t *testing.T) {
	result := buildResult("status", Outcome{Code: domain.ExitOK, Output: []byte("{\"a\":1}\n")})
	content := result["content"].([]map[string]any)
	if len(content) != 2 || content[1]["text"] != `{"a":1}` {
		t.Fatalf("content = %v", content)
	}
}

func TestDescribeAndAnnotations(t *testing.T) {
	for tier, want := range map[string]string{TierRead: "Read tool", TierWrite: "Direct write", TierPlan: "Plan tool"} {
		if got := describe(Tool{Description: "Verb.", Tier: tier}); !strings.HasPrefix(got, "Verb. "+want) {
			t.Errorf("%s: %q", tier, got)
		}
	}
	if got := describe(Tool{Description: "Verb.", Tier: "other"}); got != "Verb." {
		t.Errorf("unknown tier: %q", got)
	}
	if a := annotations(TierRead); a["readOnlyHint"] != true || a["idempotentHint"] != true || a["destructiveHint"] != false {
		t.Errorf("read annotations = %v", a)
	}
	if a := annotations(TierWrite); a["readOnlyHint"] != false || a["idempotentHint"] != false || a["openWorldHint"] != true {
		t.Errorf("write annotations = %v", a)
	}
}

func TestRequestVersion(t *testing.T) {
	if v, ok := requestVersion(nil); ok || v != "" {
		t.Fatal("no params")
	}
	if _, ok := requestVersion([]byte(`[]`)); ok {
		t.Fatal("array params")
	}
	if _, ok := requestVersion([]byte(`{"_meta":{}}`)); ok {
		t.Fatal("no version")
	}
	if v, ok := requestVersion([]byte(`{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`)); !ok || v != "2026-07-28" {
		t.Fatalf("version = %q %v", v, ok)
	}
	if v, ok := requestVersion([]byte(`{"_meta":{"io.modelcontextprotocol/protocolVersion":{"x":1}}}`)); !ok || v != `{"x":1}` {
		t.Fatalf("raw version = %q %v", v, ok)
	}
}
