package guard_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

// Rule counts are a regression gate: a change to patterns.json is a
// deliberate change of the security policy of section 6.5.
const (
	normalRules = 76
	strictRules = 27
)

// specMutations are the forbidden mutations of section 6.2.
var specMutations = []string{
	"deleteProjectV2", "deleteProjectV2Item", "deleteProjectV2Field", "deleteProjectV2View",
	"deleteProjectV2Workflow", "deleteProjectV2StatusUpdate", "archiveProjectV2Item",
	"clearProjectV2ItemFieldValue", "updateProjectV2", "updateProjectV2Field",
	"updateProjectV2Collaborators", "unlinkProjectV2FromRepository", "unmarkProjectV2AsTemplate",
	"deleteIssue", "transferIssue", "removeSubIssue", "deleteIssueComment", "updateIssueComment",
	"deleteIssueField", "deleteIssueFieldValue", "deleteIssueType", "deleteLabel", "deleteMilestone",
}

func TestPatternsAndRules(t *testing.T) {
	normal, err := guard.Patterns(false)
	if err != nil {
		t.Fatal(err)
	}
	strict, err := guard.Patterns(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(normal) != normalRules || len(strict) != normalRules+strictRules {
		t.Fatalf("normal %d strict %d, want %d and %d", len(normal), len(strict), normalRules, normalRules+strictRules)
	}
	rules, err := guard.Rules(true)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range strict {
		if rules[i] != p.Text {
			t.Errorf("rule %d: %q vs pattern %q", i, rules[i], p.Text)
		}
	}
	for _, p := range strict[normalRules:] {
		if p.Family != "strict" {
			t.Errorf("pattern %q after the normal set has family %q", p.Text, p.Family)
		}
	}
}

func TestFamilies(t *testing.T) {
	families, err := guard.Families()
	if err != nil {
		t.Fatal(err)
	}
	ids, total := []string{}, 0
	for _, f := range families {
		ids = append(ids, f.ID)
		total += len(f.Patterns)
		if f.Description == "" {
			t.Errorf("family %s without description", f.ID)
		}
	}
	if want := []string{"project", "cli", "api", "strict"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("families %v, want %v", ids, want)
	}
	if total != normalRules+strictRules {
		t.Fatalf("families hold %d rules, want %d", total, normalRules+strictRules)
	}
}

func TestForbiddenMutations(t *testing.T) {
	got, err := guard.ForbiddenMutations()
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), specMutations...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutations %v, want %v", got, want)
	}
}

var matchCases = []struct {
	rule, input string
	want        bool
}{
	{"gh project delete *", "gh project delete", true},
	{"gh project delete *", "gh project delete 999999 --owner acme", true},
	{"gh project delete *", "gh project deleted", false},
	{"gh api -X DELETE *", "gh api -X DELETE repos/acme/x/labels/bug", true},
	{"gh api * -X DELETE*", "gh api -X DELETE repos/acme/x/labels/bug", false},
	{"gh api * -X DELETE*", "gh api repos/acme/x/labels/bug -X DELETE", true},
	{"*/gh-board apply*", "/opt/bin/gh-board apply", true},
	{"*/gh-board apply*", "gh-board apply", false},
	{"gh api *deleteIssue*", "gh api graphql -f query=mutation { deleteIssue(input: {}) }", true},
	{"gh api *deleteIssue*", "gh api graphql -f query=query { deleteIssueX }", true},
}

func TestPatternMatches(t *testing.T) {
	patterns, err := guard.Patterns(true)
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]guard.Pattern{}
	for _, p := range patterns {
		byText[p.Text] = p
	}
	for _, c := range matchCases {
		p, ok := byText[c.rule]
		if !ok {
			t.Fatalf("rule %q is not in patterns.json", c.rule)
		}
		if got := p.Matches(c.input); got != c.want {
			t.Errorf("%q against %q: %v, want %v", c.rule, c.input, got, c.want)
		}
	}
}
