package guard

import (
	"errors"
	"testing"
)

// compileCases are the examples of the Claude Code permissions
// documentation, verified on 2026-10-08.
var compileCases = []struct {
	rule, input string
	want        bool
}{
	{"npm run build", "npm run build", true},
	{"npm run build", "npm run build --watch", false},
	{"npm run *", "npm run build", true},
	{"npm run *", "npm run test --watch", true},
	{"npm run *", "npm run", true},
	{"npm run *", "npm install", false},
	{"git log * main", "git log --oneline main", true},
	{"git log * main", "git log -5 main", true},
	{"git log * main", "git log main", false},
	{"git log * main", "git push origin main", false},
	{"git * main", "git merge main", true},
	{"git * main", "git -c core.fsmonitor=x diff main", true},
	{"git * main", "git log", false},
	{"* --version", "node --version", true},
	{"* --version", "bash -c 'echo hi' --version", true},
	{"* --version", "node -v", false},
	{"ls *", "ls -la", true},
	{"ls *", "ls", true},
	{"ls *", "lsof", false},
	{"ls*", "ls -la", true},
	{"ls*", "lsof", true},
	{"* --help *", "npm --help x", true},
	{"* --help *", "npm --help", false},
	{"*", "anything at all", true},
}

func TestCompileSemantics(t *testing.T) {
	for _, c := range compileCases {
		p := compile("test", c.rule)
		if got := p.Matches(c.input); got != c.want {
			t.Errorf("%q against %q: %v, want %v", c.rule, c.input, got, c.want)
		}
		if p.Family != "test" || p.Text != c.rule {
			t.Errorf("compile(%q) kept family %q text %q", c.rule, p.Family, p.Text)
		}
	}
}

var parseErrors = map[string]string{
	"json":              `{"families": [`,
	"family without id": `{"families":[{"id":"","patterns":["x *"]}],"strict":{"id":"strict","patterns":["y *"]}}`,
	"family empty":      `{"families":[{"id":"a","patterns":[]}],"strict":{"id":"strict","patterns":["y *"]}}`,
	"blank rule":        `{"families":[{"id":"a","patterns":[" "]}],"strict":{"id":"strict","patterns":["y *"]}}`,
	"duplicate":         `{"families":[{"id":"a","patterns":["x *","x *"]}],"strict":{"id":"strict","patterns":["y *"]}}`,
	"strict missing":    `{"families":[{"id":"a","patterns":["x *"]}]}`,
}

func TestParseErrors(t *testing.T) {
	for name, data := range parseErrors {
		if _, err := parse([]byte(data)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestParseSets(t *testing.T) {
	data := `{"families":[{"id":"api","patterns":["gh api *deleteIssue*","gh api -X DELETE *","gh api *a b*","gh api *x=*"]}],` +
		`"strict":{"id":"strict","patterns":["eval *"]}}`
	set, err := parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(set.normal) != 4 || len(set.strictSet) != 1 {
		t.Fatalf("normal %d strict %d", len(set.normal), len(set.strictSet))
	}
	if len(set.mutations) != 1 || set.mutations[0] != "deleteIssue" {
		t.Fatalf("mutations %v", set.mutations)
	}
	if set.strictSet[0].Family != "strict" {
		t.Fatalf("strict family %q", set.strictSet[0].Family)
	}
}

func TestEmbeddedPatternsLoad(t *testing.T) {
	set, err := loadSet()
	if err != nil {
		t.Fatal(err)
	}
	if len(set.families) != 3 || set.strict.ID != "strict" {
		t.Fatalf("families %d strict %q", len(set.families), set.strict.ID)
	}
}

var loadDependents = map[string]func() error{
	"Patterns":           func() error { _, err := Patterns(true); return err },
	"Rules":              func() error { _, err := Rules(false); return err },
	"Families":           func() error { _, err := Families(); return err },
	"ForbiddenMutations": func() error { _, err := ForbiddenMutations(); return err },
	"Evaluate":           func() error { _, _, err := Evaluate("gh project list", false); return err },
	"Check":              func() error { _, _, err := Check(AgentCursor, []byte(`{"command":"ls"}`)); return err },
	"ClaudeDenyRules":    func() error { _, err := ClaudeDenyRules(false); return err },
	"CodexRules":         func() error { _, err := CodexRules(true); return err },
}

// TestLoadFailure proves that a broken pattern source surfaces as an error
// in every entry point instead of an empty rule set that allows everything.
func TestLoadFailure(t *testing.T) {
	orig := loadSet
	t.Cleanup(func() { loadSet = orig })
	loadSet = func() (*patternSet, error) { return nil, errors.New("broken") }
	for name, fn := range loadDependents {
		if err := fn(); err == nil {
			t.Errorf("%s: expected the load error", name)
		}
	}
}
