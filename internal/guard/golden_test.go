package guard_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

var update = flag.Bool("update", false, "rewrite the golden files from the generators")

var goldenCases = []struct {
	name, file string
	gen        func() ([]byte, error)
}{
	{"claude deny", "claude-deny.golden", func() ([]byte, error) { return lines(guard.ClaudeDenyRules(false)) }},
	{"claude deny strict", "claude-deny-strict.golden", func() ([]byte, error) { return lines(guard.ClaudeDenyRules(true)) }},
	{"codex rules", "codex.rules", func() ([]byte, error) { return guard.CodexRules(false) }},
	{"codex rules strict", "codex-strict.rules", func() ([]byte, error) { return guard.CodexRules(true) }},
}

func lines(rules []string, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(rules, "\n") + "\n"), nil
}

func TestGolden(t *testing.T) {
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.gen()
			if err != nil {
				t.Fatal(err)
			}
			compareGolden(t, filepath.Join("testdata", c.file), got)
		})
	}
}

func compareGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lf(got), lf(want)) {
		t.Fatalf("%s differs from the golden file; run: go test ./internal/guard -run Golden -update\n%s", path, got)
	}
}

var prefixCases = []struct {
	rule string
	want []string
}{
	{"gh project delete *", []string{"gh", "project", "delete"}},
	{"eval *", []string{"eval"}},
	{"gh api -X DELETE *", []string{"gh", "api", "-X", "DELETE"}},
	{"gh api * -X DELETE*", nil},
	{"*/gh-board apply*", nil},
	{"gh api --input=*", nil},
	{"gh api *deleteIssue*", nil},
	{" *", nil},
	{"ls*", nil},
}

func TestPrefixTokens(t *testing.T) {
	for _, c := range prefixCases {
		got, ok := guard.PrefixTokens(c.rule)
		if ok != (c.want != nil) || strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("PrefixTokens(%q) = %q %v, want %q", c.rule, got, ok, c.want)
		}
	}
	if _, ok := guard.CodexPrefix("bash -lc *"); ok {
		t.Error("CodexPrefix must refuse the shells")
	}
	if tokens, ok := guard.CodexPrefix("gh repo delete *"); !ok || strings.Join(tokens, " ") != "gh repo delete" {
		t.Errorf("CodexPrefix(gh repo delete *) = %q %v", tokens, ok)
	}
}

func TestCodexRulesContent(t *testing.T) {
	data, err := guard.CodexRules(true)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if got := strings.Count(text, "prefix_rule("); got != 725 {
		t.Fatalf("%d strict prefix rules, want 725", got)
	}
	for _, needle := range []string{`pattern=["gh", "project", "delete"], decision="forbidden", justification="gh board guard denies gh project delete")`, `pattern=["eval"],`, `pattern=["env", "gh"],`, "# Strict mode: on"} {
		if !strings.Contains(text, needle) {
			t.Errorf("rules lack %s", needle)
		}
	}
	if strings.Contains(text, `["bash", "-lc"]`) || strings.Contains(text, `["sh", "-c"]`) {
		t.Fatal("the shells Codex runs commands with must never be forbidden prefixes")
	}
	normal, _ := guard.CodexRules(false)
	if strings.Contains(string(normal), `["eval"]`) || !strings.Contains(string(normal), "# Strict mode: off") {
		t.Fatal("normal rules must not carry the strict prefixes")
	}
}

// shippedCodexRules is the rules file the kit embeds and the installer
// writes; it must be exactly what the generator produces.
const shippedCodexRules = "../../agents/codex/gh-board.rules"

func TestShippedCodexRulesMatchGenerator(t *testing.T) {
	want, err := guard.CodexRules(false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(shippedCodexRules)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lf(got), lf(want)) {
		t.Fatalf("%s differs from guard.CodexRules(false); copy internal/guard/testdata/codex.rules over it", shippedCodexRules)
	}
}

func lf(data []byte) []byte { return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")) }

func TestGoldenCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crlf.golden")
	if err := os.WriteFile(path, []byte("one\r\ntwo\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, path, []byte("one\ntwo\n"))
}
