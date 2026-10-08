package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

const (
	acmeProjectID   = "PVT_6gozs2PV0g9mjnL3"
	firstItemID     = "PVTI_HFqu-DJTxb8Yy5MzJtRxKu-"
	firstIssueID    = "I_2Gxqm1o_-TlPmXqb57tKlQ"
	firstPageCursor = "Y3Vyc29yOnYyOpK5MDAwMDAwMDAuMDE5MjMwNzY5MjMwNzY5Ms4PDFMP"
)

var acmeProject = domain.Project{Ref: acmeBoard, NodeID: acmeProjectID}

func TestListItemsFirstPage(t *testing.T) {
	r, a := newReplay(t, "acme")
	page, err := a.ListItems(context.Background(), acmeProject, domain.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 12 || !page.HasNext || page.NextCursor != firstPageCursor || page.TotalCount != 169 {
		t.Fatalf("page = %d items, next %v %q, total %d", len(page.Items), page.HasNext, page.NextCursor, page.TotalCount)
	}
	vars := r.last().Variables
	if vars["first"] != float64(100) || vars["after"] != nil || vars["project"] != acmeProjectID || vars["full"] != true {
		t.Fatalf("variables = %v", vars)
	}
	assertFirstItem(t, page.Items[0])
}

// TestListItemsSummarySelection checks the switch the summary selection
// sends; the replay answers the recorded page either way.
func TestListItemsSummarySelection(t *testing.T) {
	r, a := newReplay(t, "acme")
	page, err := a.ListItems(context.Background(), acmeProject, domain.ListOptions{Selection: domain.SelectionSummary})
	if err != nil || len(page.Items) != 12 {
		t.Fatalf("summary page: %v, %d items", err, len(page.Items))
	}
	if vars := r.last().Variables; vars["full"] != false || vars["first"] != float64(100) {
		t.Fatalf("variables = %v", vars)
	}
	if _, err := a.ListItems(context.Background(), acmeProject, domain.ListOptions{Selection: domain.SelectionFull}); err != nil || r.last().Variables["full"] != true {
		t.Fatalf("full selection: %v, variables %v", err, r.last().Variables)
	}
}

func assertFirstItem(t *testing.T, it domain.Item) {
	t.Helper()
	issue := it.Issue
	if it.ProjectItemID != firstItemID || it.Archived || issue.NodeID != firstIssueID || issue.Ref() != "acme/app#126" {
		t.Fatalf("identity = %+v", it)
	}
	if issue.State != domain.IssueOpen || issue.Title == "" || issue.URL == "" || issue.Type == nil || issue.Type.Name != "Task" {
		t.Fatalf("issue = %+v", issue)
	}
	if issue.Parent == nil || issue.Parent.Number != 155 || issue.Parent.Repo != "app" || issue.Parent.NodeID == "" {
		t.Fatalf("parent = %+v", issue.Parent)
	}
	if len(issue.Assignees) != 3 || !it.AssignedTo("MEMBRO2") || len(issue.Labels) != 3 || !it.HasLabel("Plataforma-Dados") {
		t.Fatalf("assignees %v labels %v", issue.Assignees, issue.Labels)
	}
	if issue.Milestone == nil || issue.Milestone.Title != "Marco 1" || issue.Milestone.Number != 8 || issue.Milestone.DueOn == nil {
		t.Fatalf("milestone = %+v", issue.Milestone)
	}
	if issue.BlockedByCount != 0 || issue.SubIssues.Total != 0 || issue.CreatedAt.IsZero() || issue.ClosedAt != nil {
		t.Fatalf("counters = %+v", issue)
	}
	assertFirstItemValues(t, it)
}

func assertFirstItemValues(t *testing.T, it domain.Item) {
	t.Helper()
	status, ok := it.Value("Status")
	if !ok || status.Value != "IN PROGRESS" || status.OptionID != "97bab87a" || status.Creator != "github-project-automation" {
		t.Fatalf("Status = %+v", status)
	}
	if !status.UpdatedAt.Equal(time.Date(2026, 10, 6, 18, 59, 28, 0, time.UTC)) {
		t.Fatalf("Status.UpdatedAt = %s", status.UpdatedAt)
	}
	sprint, _ := it.Value("Sprint")
	if sprint.Value != "Sprint 5" || sprint.IterationID != "05188f6c" || sprint.Creator != "membro1" {
		t.Fatalf("Sprint = %+v", sprint)
	}
	if it.Text("Estimativa (dias)") != "0.5" || it.Text("Start date") != "2026-10-06" || it.Text("Title") == "" {
		t.Fatalf("values = %+v", it.Values)
	}
	if _, ok := it.Value("Labels"); ok {
		t.Fatal("members without a selection are not values")
	}
	target, ok := it.IssueField("Target date")
	if !ok || target.Value != "2026-10-07" || target.FieldID != "IFD_QPgBnZsICL" || len(it.IssueFields) != 2 {
		t.Fatalf("issue fields = %+v", it.IssueFields)
	}
}

func TestListItemsNextPageAndAll(t *testing.T) {
	r, a := newReplay(t, "acme")
	ctx := context.Background()
	second, err := a.ListItems(ctx, acmeProject, domain.ListOptions{Cursor: firstPageCursor})
	if err != nil || len(second.Items) != 12 || !second.HasNext || r.last().Variables["after"] != firstPageCursor {
		t.Fatalf("second page: %v, %d items, after %v", err, len(second.Items), r.last().Variables["after"])
	}
	all, err := a.ListItems(ctx, acmeProject, domain.ListOptions{All: true})
	if err != nil || len(all.Items) != 48 || all.HasNext || all.NextCursor != "" || all.TotalCount != 169 {
		t.Fatalf("all: %v, %d items, next %v", err, len(all.Items), all.HasNext)
	}
	if r.count() != 5 {
		t.Fatalf("requests = %d, want 1 + 4 pages", r.count())
	}
}

// filterCases pair an adapter filter with the domain predicate it must
// equal; the expected count comes from the unfiltered items.
var filterCases = []struct {
	name   string
	filter domain.ItemFilter
	keep   func(domain.Item) bool
}{
	{"closed", domain.ItemFilter{State: domain.IssueClosed}, func(it domain.Item) bool { return !it.IsOpen() }},
	{"assignee ignoring case", domain.ItemFilter{Assignee: "MEMBRO1"}, func(it domain.Item) bool { return it.AssignedTo("membro1") }},
	{"label ignoring case", domain.ItemFilter{Label: "Plataforma-Dados"}, func(it domain.Item) bool { return it.HasLabel("plataforma-dados") }},
	{"type ignoring case", domain.ItemFilter{Type: "feature"}, func(it domain.Item) bool { return it.Issue.Type != nil && it.Issue.Type.Name == "Feature" }},
	{"nobody", domain.ItemFilter{Assignee: "nobody"}, func(domain.Item) bool { return false }},
	{"combined", domain.ItemFilter{Type: "Task", Label: "plataforma-dados", State: domain.IssueOpen}, func(it domain.Item) bool {
		return it.IsOpen() && it.HasLabel("plataforma-dados") && it.Issue.Type != nil && it.Issue.Type.Name == "Task"
	}},
}

func TestListItemsFilters(t *testing.T) {
	_, a := newReplay(t, "acme")
	ctx := context.Background()
	all, err := a.ListItems(ctx, acmeProject, domain.ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range filterCases {
		t.Run(tc.name, func(t *testing.T) {
			want := countKept(all.Items, tc.keep)
			if tc.name != "nobody" && (want == 0 || want == len(all.Items)) {
				t.Fatalf("the fixture must hold a strict subset, got %d of %d", want, len(all.Items))
			}
			page, err := a.ListItems(ctx, acmeProject, domain.ListOptions{All: true, Filter: tc.filter})
			if err != nil || len(page.Items) != want {
				t.Fatalf("got %d items, err %v, want %d", len(page.Items), err, want)
			}
		})
	}
}

func countKept(items []domain.Item, keep func(domain.Item) bool) int {
	n := 0
	for _, it := range items {
		if keep(it) {
			n++
		}
	}
	return n
}

func TestListItemsArchivedAndLimits(t *testing.T) {
	r, a := newReplay(t, "acme", "template")
	ctx := context.Background()
	archived, err := a.ListItems(ctx, acmeProject, domain.ListOptions{Filter: domain.ItemFilter{Archived: true}})
	if err != nil || len(archived.Items) != 0 || archived.TotalCount != 4 || archived.HasNext {
		t.Fatalf("archived drafts are skipped: %v, %+v", err, archived)
	}
	if states, _ := r.last().Variables["archived"].([]any); len(states) != 1 || states[0] != "ARCHIVED" {
		t.Fatalf("archived variable = %v", r.last().Variables["archived"])
	}
	if _, err := a.ListItems(ctx, acmeProject, domain.ListOptions{Limit: 5}); err != nil || r.last().Variables["first"] != float64(5) {
		t.Fatalf("limit: %v, first %v", err, r.last().Variables["first"])
	}
	if _, err := a.ListItems(ctx, acmeProject, domain.ListOptions{Limit: 500}); err != nil || r.last().Variables["first"] != float64(100) {
		t.Fatalf("limit cap: %v, first %v", err, r.last().Variables["first"])
	}
	empty, err := a.ListItems(ctx, domain.Project{NodeID: "PVT_VDlOjkUW8bidBVTR"}, domain.ListOptions{All: true})
	if err != nil || len(empty.Items) != 0 || empty.Items == nil || empty.HasNext {
		t.Fatalf("empty board: %v, %+v", err, empty)
	}
}

func TestListItemsMissingProject(t *testing.T) {
	_, a := newReplay(t, "errors")
	_, err := a.ListItems(context.Background(), domain.Project{NodeID: "PVT_missingProject00"}, domain.ListOptions{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

var getItemCases = []struct {
	name string
	ref  string
	want error
}{
	{"project item node", firstItemID, nil},
	{"issue node", firstIssueID, nil},
	{"project node", acmeProjectID, domain.ErrNotFound},
	{"full reference", "acme/app#126", nil},
	{"url", "https://github.com/acme/app/issues/126", nil},
	{"missing number", "acme/app#999999", domain.ErrNotFound},
	{"bare number", "#126", nil},
	{"bare number missing", "999999", domain.ErrNotFound},
}

func TestGetItem(t *testing.T) {
	for _, tc := range getItemCases {
		t.Run(tc.name, func(t *testing.T) {
			_, a := newReplay(t, "acme")
			ref, err := domain.ParseReference(tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			it, err := a.GetItem(context.Background(), acmeProject, ref)
			if tc.want != nil {
				if !errors.Is(err, tc.want) || domain.CodeOf(err) != domain.ExitNotFound {
					t.Fatalf("expected %v, got %v", tc.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertFirstItem(t, it)
		})
	}
}

// TestGetItemBareNumberReadsSummaries checks that a bare number walks the
// summaries of the board and then reads the one item in full by its node
// id.
func TestGetItemBareNumberReadsSummaries(t *testing.T) {
	r, a := newReplay(t, "acme")
	it, err := a.GetItem(context.Background(), acmeProject, domain.Reference{Kind: domain.RefNumber, Number: 126})
	if err != nil {
		t.Fatal(err)
	}
	assertFirstItem(t, it)
	calls := r.all()
	if len(calls) != 5 || calls[4].Operation != "ItemByNode" || calls[4].Variables["id"] != firstItemID {
		t.Fatalf("calls = %+v", calls)
	}
	for _, c := range calls[:4] {
		if c.Operation != "ListItems" || c.Variables["full"] != false {
			t.Fatalf("the lookup must read summaries: %+v", c)
		}
	}
}

func TestGetItemEdges(t *testing.T) {
	_, a := newReplay(t, "acme", "ambiguous")
	ctx := context.Background()
	other := domain.Project{Ref: domain.ProjectRef{Owner: "acme", Number: 1}, NodeID: "PVT_someOtherBoard000"}
	if _, err := a.GetItem(ctx, other, domain.Reference{Kind: domain.RefNodeID, NodeID: firstItemID}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("an item of another board is not found: %v", err)
	}
	if _, err := a.GetItem(ctx, acmeProject, domain.Reference{}); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a reference without kind is a usage error: %v", err)
	}
	twins := domain.Project{Ref: acmeBoard, NodeID: "PVT_ambiguousBoard0000"}
	_, err := a.GetItem(ctx, twins, domain.Reference{Kind: domain.RefNumber, Number: 126})
	if !errors.Is(err, domain.ErrAmbiguous) || domain.CodeOf(err) != domain.ExitNotFound {
		t.Fatalf("two issues with the same number are ambiguous: %v", err)
	}
}
