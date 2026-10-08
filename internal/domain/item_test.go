package domain_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var sampleItem = domain.Item{
	ProjectItemID: "PVTI_1",
	Issue: domain.Issue{
		NodeID: "I_1", Owner: "acme", Repo: "app", Number: 12, State: domain.IssueOpen,
		Labels:    []domain.Label{{Name: "entrada"}, {Name: "Urgente"}},
		Assignees: []domain.User{{Login: "ana"}},
	},
	Values: map[string]domain.FieldValue{
		"Status": {Field: "Status", Value: "IN PROGRESS", OptionID: "o3", UpdatedAt: time.Unix(0, 0)},
	},
	IssueFields: []domain.IssueFieldValue{{FieldID: "IF_1", Name: "Target date", Value: "2026-10-30"}},
}

func TestIssueRef(t *testing.T) {
	if got := sampleItem.Issue.Ref(); got != "acme/app#12" {
		t.Fatalf("Ref() = %q", got)
	}
}

func TestItemValues(t *testing.T) {
	v, ok := sampleItem.Value("Status")
	if !ok || v.OptionID != "o3" {
		t.Fatalf("Value = %+v, %v", v, ok)
	}
	if _, ok := sampleItem.Value("Sprint"); ok {
		t.Fatal("unset field must not be found")
	}
	if sampleItem.Text("Status") != "IN PROGRESS" || sampleItem.Text("Sprint") != "" {
		t.Fatal("Text is wrong")
	}
	target, ok := sampleItem.IssueField("Target date")
	if !ok || target.Value != "2026-10-30" {
		t.Fatalf("IssueField = %+v, %v", target, ok)
	}
	if _, ok := sampleItem.IssueField("Start date"); ok {
		t.Fatal("missing issue field must not be found")
	}
}

func TestItemPredicates(t *testing.T) {
	if !sampleItem.HasLabel("urgente") || sampleItem.HasLabel("lab") {
		t.Fatal("HasLabel is wrong")
	}
	if !sampleItem.AssignedTo("Ana") || sampleItem.AssignedTo("nobody") {
		t.Fatal("AssignedTo is wrong")
	}
	if !sampleItem.IsOpen() {
		t.Fatal("IsOpen is wrong")
	}
	closed := sampleItem
	closed.Issue.State = domain.IssueClosed
	if closed.IsOpen() {
		t.Fatal("closed item reported open")
	}
}

var viewerCases = []struct {
	source string
	want   bool
}{
	{"GH_TOKEN", true}, {"GITHUB_TOKEN", true}, {"oauth_token", false}, {"gh", false}, {"default", false},
}

func TestViewerShadowed(t *testing.T) {
	for _, tc := range viewerCases {
		v := domain.Viewer{Login: "me", TokenSource: tc.source}
		if v.Shadowed() != tc.want {
			t.Errorf("%s: Shadowed() = %v", tc.source, v.Shadowed())
		}
	}
}

var permissionCases = map[domain.Permission]bool{
	domain.PermissionAdmin: true, domain.PermissionMaintain: true, domain.PermissionWrite: true,
	domain.PermissionTriage: true, domain.PermissionRead: false, domain.PermissionNone: false, "": false,
}

func TestPermissionCanWrite(t *testing.T) {
	for p, want := range permissionCases {
		if p.CanWrite() != want {
			t.Errorf("%q.CanWrite() = %v", p, p.CanWrite())
		}
	}
}
