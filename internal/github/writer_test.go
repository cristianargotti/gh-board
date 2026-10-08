package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

// Identities of the sandbox recordings (sanitized ids).
const (
	sandboxProjectID = "PVT_wNsk5eGXHNnHmD7K"
	sandboxRepoID    = "R_pnZsM5O_6f"
	sandboxIssueID   = "I_vWR8nnSf6Ch6ckP3dRPc2t"
	sandboxParentID  = "I_pUL4m1pgFuLYPyYqu6crK-"
	sandboxItemID    = "PVTI_Q0Wn5uLXWpGEnAXpSCIPgh5"
	sandboxParentItm = "PVTI_Hb2IyeQTZLqeIkI4hezZd4d"
	sandboxLabelID   = "LA_z4YTG_lw2ByXgGJpObmQg9"
	sandboxViewerID  = "U_2JRxF9gBtX"
	sandboxMilestone = "MI_iOIuVKTXnBfviszE"
	statusFieldID    = "PVTSSF_Y3It7eBVsqHLNATKgRLkikG"
	numberFieldID    = "PVTF_vTyGCYodJ7dLcS-V-WA_Rpc"
	dateFieldID      = "PVTF_chruvtrl9xjOY8WvOgJ-GLx"
	iterationFieldID = "PVTIF_Fw8OwXKQkYAXzXLF0aUgXi6"
	copiedProjectID  = "PVT_VORNCTezFznZfm0B"
)

func TestCreateAndUpdateIssue(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	issue, err := a.CreateIssue(ctx, domain.CreateIssueInput{
		RepositoryID: sandboxRepoID, Title: "Tarefa de teste do kit", Body: "Criada pelo adapter em teste.",
		LabelIDs: []string{sandboxLabelID}, AssigneeIDs: []string{sandboxViewerID},
	})
	if err != nil || issue.NodeID != sandboxIssueID || issue.Number != 1 || issue.Ref() != "membro1/gh-board-sandbox#1" {
		t.Fatalf("create: %v, %+v", err, issue)
	}
	if len(issue.Labels) != 1 || issue.Labels[0].Name != "kit-teste" || len(issue.Assignees) != 1 || issue.State != domain.IssueOpen {
		t.Fatalf("created issue = %+v", issue)
	}
	vars := r.last().Variables
	if vars["issueTypeId"] != nil || vars["milestoneId"] != nil || vars["body"] == nil {
		t.Fatalf("empty members must be omitted: %v", vars)
	}
	title, body := "Tarefa de teste do kit (editada)", "Corpo editado pelo teste."
	edited, err := a.UpdateIssue(ctx, domain.UpdateIssueInput{IssueID: sandboxIssueID, Title: &title, Body: &body})
	if err != nil || edited.NodeID != sandboxIssueID || r.last().Variables["title"] != title || r.last().Variables["milestoneId"] != nil {
		t.Fatalf("update: %v, %+v, vars %v", err, edited, r.last().Variables)
	}
	milestone := sandboxMilestone
	withMilestone, err := a.UpdateIssue(ctx, domain.UpdateIssueInput{IssueID: sandboxIssueID, MilestoneID: &milestone})
	if err != nil || withMilestone.Milestone == nil || withMilestone.Milestone.ID != sandboxMilestone || r.last().Variables["title"] != nil {
		t.Fatalf("milestone: %v, %+v", err, withMilestone.Milestone)
	}
}

func TestIssueSetMutations(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	steps := []struct {
		name string
		call func() error
		list string
	}{
		{"remove_assignees", func() error { return a.RemoveAssignees(ctx, sandboxIssueID, []string{sandboxViewerID}) }, "assigneeIds"},
		{"add_assignees", func() error { return a.AddAssignees(ctx, sandboxIssueID, []string{sandboxViewerID}) }, "assigneeIds"},
		{"remove_labels", func() error { return a.RemoveLabels(ctx, sandboxIssueID, []string{sandboxLabelID}) }, "labelIds"},
		{"add_labels", func() error { return a.AddLabels(ctx, sandboxIssueID, []string{sandboxLabelID}) }, "labelIds"},
		{"close_issue", func() error { return a.CloseIssue(ctx, sandboxIssueID) }, ""},
		{"reopen_issue", func() error { return a.ReopenIssue(ctx, sandboxIssueID) }, ""},
		{"add_sub_issue", func() error { return a.AddSubIssue(ctx, sandboxParentID, sandboxIssueID) }, ""},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			if err := step.call(); err != nil {
				t.Fatal(err)
			}
			if ids, _ := r.last().Variables[step.list].([]any); step.list != "" && len(ids) != 1 {
				t.Fatalf("variables = %v", r.last().Variables)
			}
		})
	}
	if last := r.last().Variables; last["issueId"] != sandboxParentID || last["subIssueId"] != sandboxIssueID {
		t.Fatalf("sub-issue variables = %v", last)
	}
}

func TestAddCommentAndErrors(t *testing.T) {
	_, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	c, err := a.AddComment(ctx, sandboxIssueID, "Comentário de teste do kit.")
	if err != nil || c.ID == "" || c.Author != "membro1" || c.URL == "" || c.CreatedAt.IsZero() {
		t.Fatalf("comment: %v, %+v", err, c)
	}
	err = a.UpdateIssueType(ctx, sandboxIssueID, "IT_kwDOACm68c4AAAAA")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("an unknown issue type is reported by GitHub as not found: %v", err)
	}
}

func TestIssueFieldValues(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes", "acme")
	ctx := context.Background()
	in := domain.IssueFieldValueInput{IssueID: firstIssueID, FieldID: "IFD_e7jmu-_8WZ", Value: "2026-10-10"}
	if err := a.SetIssueFieldValue(ctx, in); err != nil {
		t.Fatal(err)
	}
	fields, _ := r.last().Variables["issueFields"].([]any)
	field, _ := fields[0].(map[string]any)
	if len(fields) != 1 || field["fieldId"] != in.FieldID || field["dateValue"] != "2026-10-10" || field["delete"] != nil {
		t.Fatalf("issueFields = %v", fields)
	}
	if err := a.UpdateIssueFieldValue(ctx, in); err != nil {
		t.Fatal(err)
	}
	if single, _ := r.last().Variables["issueField"].(map[string]any); single["dateValue"] != "2026-10-10" {
		t.Fatalf("issueField = %v", r.last().Variables["issueField"])
	}
	bad := domain.IssueFieldValueInput{IssueID: firstIssueID, FieldID: "IFD_e7jmu-_8WZ", Value: "soon"}
	if err := a.SetIssueFieldValue(ctx, bad); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a bad date is a usage error: %v", err)
	}
	forbidden := domain.IssueFieldValueInput{IssueID: sandboxIssueID, FieldID: "IFD_fakeDateField00", Value: "2026-10-10"}
	if err := a.SetIssueFieldValue(ctx, forbidden); !errors.Is(err, domain.ErrAPI) {
		t.Fatalf("a user repository refuses organization fields: %v", err)
	}
	missing := domain.IssueFieldValueInput{IssueID: sandboxIssueID, FieldID: acmeProjectID, Value: "2026-10-10"}
	if err := a.UpdateIssueFieldValue(ctx, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a node that is not an issue field: %v", err)
	}
}

func TestUpdateItemFieldValue(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	option, iteration, text := "f75ad846", "0b1c2d3e", "x"
	number := 1.5
	date := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		in    domain.ItemFieldValueInput
		key   string
		value any
	}{
		{"status", domain.ItemFieldValueInput{FieldID: statusFieldID, SingleSelectID: &option}, "singleSelectOptionId", option},
		{"number", domain.ItemFieldValueInput{FieldID: numberFieldID, Number: &number}, "number", number},
		{"date", domain.ItemFieldValueInput{FieldID: dateFieldID, Date: &date}, "date", "2026-10-10"},
		{"iteration", domain.ItemFieldValueInput{FieldID: iterationFieldID, IterationID: &iteration}, "iterationId", iteration},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.ProjectID, tc.in.ItemID = sandboxProjectID, sandboxItemID
			if err := a.UpdateItemFieldValue(ctx, tc.in); err != nil {
				t.Fatal(err)
			}
			value, _ := r.last().Variables["value"].(map[string]any)
			if len(value) != 1 || value[tc.key] != tc.value {
				t.Fatalf("value = %v", value)
			}
		})
	}
	two := domain.ItemFieldValueInput{ProjectID: sandboxProjectID, ItemID: sandboxItemID, FieldID: statusFieldID, Text: &text, Number: &number}
	if err := a.UpdateItemFieldValue(ctx, two); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("two members: %v", err)
	}
}

func TestProjectMutations(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	itemID, err := a.AddProjectItem(ctx, sandboxProjectID, sandboxIssueID)
	if err != nil || itemID != sandboxItemID {
		t.Fatalf("add item: %v, %q", err, itemID)
	}
	if err := a.UnarchiveItem(ctx, sandboxProjectID, sandboxParentItm); err != nil {
		t.Fatal(err)
	}
	start, target := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	updateID, err := a.CreateStatusUpdate(ctx, domain.StatusUpdateInput{ProjectID: sandboxProjectID, Body: "Semana de teste do kit.", Status: "on_track", StartDate: &start, TargetDate: &target})
	if err != nil || updateID == "" {
		t.Fatalf("status update: %v, %q", err, updateID)
	}
	if vars := r.last().Variables; vars["status"] != "ON_TRACK" || vars["startDate"] != "2026-10-06" || vars["targetDate"] != "2026-10-17" {
		t.Fatalf("status update variables = %v", vars)
	}
	field, err := a.CreateProjectField(ctx, domain.CreateFieldInput{ProjectID: sandboxProjectID, Name: "Campo de teste", DataType: domain.DataTypeSingleSelect, SingleSelectOptions: []string{"Opção A", "Opção B"}})
	if err != nil || field.Name != "Campo de teste" || field.DataType != domain.DataTypeSingleSelect || len(field.Options) != 2 || field.ID == "" {
		t.Fatalf("field: %v, %+v", err, field)
	}
	options, _ := r.last().Variables["singleSelectOptions"].([]any)
	first, _ := options[0].(map[string]any)
	if len(options) != 2 || first["name"] != "Opção A" || first["color"] != "GRAY" || first["description"] != "" {
		t.Fatalf("options = %v", options)
	}
	label, err := a.CreateLabel(ctx, domain.CreateLabelInput{RepositoryID: sandboxRepoID, Name: "kit-teste", Color: "0e8a16", Description: "Etiqueta de teste do kit"})
	if err != nil || label.ID != sandboxLabelID || label.Color != "0e8a16" {
		t.Fatalf("label: %v, %+v", err, label)
	}
}

func TestCopyProjectAndLink(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	copied, err := a.CopyProject(ctx, domain.CopyProjectInput{SourceProjectID: sandboxProjectID, OwnerID: sandboxViewerID, Title: "Cópia de teste do kit"})
	if err != nil {
		t.Fatal(err)
	}
	if copied.NodeID != copiedProjectID || copied.Ref != (domain.ProjectRef{Owner: "membro1", Number: 3}) || copied.ViewerRole != domain.RoleAdmin {
		t.Fatalf("copied = %+v", copied)
	}
	if len(copied.Fields) == 0 || len(copied.Views) == 0 || copied.ItemCount != 0 || !copied.DiscoveredAt.Equal(testNow) {
		t.Fatalf("copied schema = %d fields, %d views, %d items", len(copied.Fields), len(copied.Views), copied.ItemCount)
	}
	if r.last().Variables["includeDraftIssues"] != false {
		t.Fatalf("variables = %v", r.last().Variables)
	}
	if err := a.LinkProjectToRepository(ctx, copiedProjectID, sandboxRepoID); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMilestoneThroughREST(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	due := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	m, err := a.CreateMilestone(context.Background(), domain.CreateMilestoneInput{Owner: "membro1", Repo: "gh-board-sandbox", Title: "Marco de teste do kit", Description: "Marco criado pelo teste do adapter.", DueOn: &due})
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != sandboxMilestone || m.Number != 1 || m.State != "OPEN" || m.DueOn == nil || m.Title != "Marco de teste do kit" {
		t.Fatalf("milestone = %+v", m)
	}
	last := r.last()
	if last.Method != "POST" || last.Path != "/repos/membro1/gh-board-sandbox/milestones" || last.Body["due_on"] != "2026-12-31T12:00:00Z" || last.Body["title"] != m.Title {
		t.Fatalf("request = %+v", last)
	}
	_, err = a.CreateMilestone(context.Background(), domain.CreateMilestoneInput{Owner: "membro1", Repo: "other", Title: "x"})
	var apiErr *github.APIError
	if !errors.Is(err, domain.ErrAPI) || !errors.As(err, &apiErr) || apiErr.Status != 500 {
		t.Fatalf("an unrecorded path answers an API error: %v", err)
	}
}
