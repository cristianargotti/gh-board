package github

import (
	"regexp"
	"strings"
	"testing"
)

var definedFragmentPattern = regexp.MustCompile(`(?m)^fragment\s+(\w+)\s+on`)

var documentFragmentCases = []struct {
	name      string
	fragments []string
}{
	{"viewer", nil},
	{"viewer_permission", nil},
	{"issue_field", nil},
	{"discover_project", []string{"ProjectSchema"}},
	{"copy_project", []string{"ProjectSchema"}},
	{"create_issue", []string{"IssueFields", "IssueSummary", "IssueDetails", "IssueFieldValueParts", "IssueFieldRef"}},
	{"update_issue", []string{"IssueFields", "IssueSummary", "IssueDetails", "IssueFieldValueParts", "IssueFieldRef"}},
	{"list_items", []string{"ItemCore", "ItemIssueFields", "IssueSummary", "IssueDetails", "IssueFieldValueParts", "IssueFieldRef"}},
	{"item_by_node", []string{"ItemFields", "ItemCore", "ItemIssueFields", "IssueFields", "IssueSummary", "IssueDetails", "IssueFieldValueParts", "IssueFieldRef"}},
	{"issue_by_number", []string{"ItemFields", "ItemCore", "ItemIssueFields", "IssueFields", "IssueSummary", "IssueDetails", "IssueFieldValueParts", "IssueFieldRef"}},
	{"issue_by_id", []string{"IssueFields", "IssueSummary", "IssueDetails", "IssueFieldValueParts", "IssueFieldRef"}},
	{"project_by_id", []string{"ProjectSchema"}},
	{"organization_issue_fields", []string{"IssueFieldRef"}},
	{"issue_timeline", []string{"IssueFieldRef"}},
	{"user_id", nil},
	{"owner_id", nil},
	{"repository_id", nil},
	{"assignable_users", nil},
	{"status_updates", nil},
	{"release_assets", nil},
}

func TestDocumentComposesOnlySpreadFragments(t *testing.T) {
	for _, tc := range documentFragmentCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := document(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			assertFragments(t, doc, tc.fragments)
		})
	}
}

func assertFragments(t *testing.T, doc string, want []string) {
	t.Helper()
	var defined []string
	for _, m := range definedFragmentPattern.FindAllStringSubmatch(doc, -1) {
		defined = append(defined, m[1])
	}
	if strings.Join(defined, ",") != strings.Join(want, ",") {
		t.Fatalf("fragments = %v, want %v", defined, want)
	}
	for _, name := range defined {
		if !strings.Contains(doc, "..."+name) {
			t.Fatalf("fragment %s is defined but never spread", name)
		}
	}
}

func TestEveryDocumentIsOneOperationWithVariablesOnly(t *testing.T) {
	documentsOnce.Do(loadDocuments)
	if documentsErr != nil {
		t.Fatal(documentsErr)
	}
	if len(documents) != 42 {
		t.Fatalf("embedded %d documents, want 42", len(documents))
	}
	operation := regexp.MustCompile(`(?m)^(query|mutation)\s+\w+`)
	for name, doc := range documents {
		if n := len(operation.FindAllString(doc, -1)); n != 1 {
			t.Errorf("%s declares %d operations", name, n)
		}
		if strings.Contains(doc, `"`) {
			t.Errorf("%s carries a string literal; documents use variables only", name)
		}
	}
}

// The list document carries the selection switch on the parts the
// summary leaves out, and only there.
func TestListDocumentSwitchesTheDetails(t *testing.T) {
	doc, err := document("list_items")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"$full: Boolean!", "...ItemIssueFields @include(if: $full)", "...IssueDetails @include(if: $full)"} {
		if strings.Count(doc, want) != 1 {
			t.Errorf("list_items must carry %q once:\n%s", want, doc)
		}
	}
	if strings.Count(doc, "@include") != 2 {
		t.Errorf("list_items must switch exactly the two detail fragments:\n%s", doc)
	}
	for _, name := range []string{"item_by_node", "issue_by_number", "issue_by_id"} {
		other, err := document(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(other, "@include") || strings.Contains(other, "$full") {
			t.Errorf("%s must read everything without a switch", name)
		}
	}
}

func TestDocumentUnknown(t *testing.T) {
	if _, err := document("no_such_document"); err == nil {
		t.Fatal("expected an error for an unknown document")
	}
}

func TestSplitFragmentsMatchesBraces(t *testing.T) {
	src := "fragment A on X { a { b } }\n# note\nfragment B on Y {\n  c\n}\n"
	got := splitFragments(src)
	if got["A"] != "fragment A on X { a { b } }" {
		t.Fatalf("A = %q", got["A"])
	}
	if got["B"] != "fragment B on Y {\n  c\n}" {
		t.Fatalf("B = %q", got["B"])
	}
	composed := compose("query Q { ...B ...A }", got)
	if !strings.HasSuffix(composed, got["A"]) || !strings.Contains(composed, got["B"]) {
		t.Fatalf("compose = %q", composed)
	}
	if compose("query Q { ... on X { a } }", got) != "query Q { ... on X { a } }" {
		t.Fatal("inline fragments must not pull named ones")
	}
}
