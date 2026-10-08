package plan_test

import (
	"context"
	"strconv"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// fakeWriter records every mutation as "Method arg arg" and returns the
// fixtures it holds; fail injects an error per method name.
type fakeWriter struct {
	calls     []string
	fail      map[string]error
	issue     domain.Issue
	itemID    string
	comment   domain.Comment
	updateID  string
	field     domain.Field
	label     domain.Label
	milestone domain.Milestone
	project   domain.Project
	updates   []domain.StatusUpdateInput
}

func (w *fakeWriter) call(name string, args ...string) error {
	w.calls = append(w.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	return w.fail[name]
}

func deref(s *string) string {
	if s == nil {
		return "nil"
	}
	return *s
}

func (w *fakeWriter) CreateIssue(_ context.Context, in domain.CreateIssueInput) (domain.Issue, error) {
	return w.issue, w.call("CreateIssue", in.RepositoryID, in.Title, in.IssueTypeID)
}

func (w *fakeWriter) UpdateIssue(_ context.Context, in domain.UpdateIssueInput) (domain.Issue, error) {
	return w.issue, w.call("UpdateIssue", in.IssueID, deref(in.Title), deref(in.Body), deref(in.MilestoneID))
}

func (w *fakeWriter) AddAssignees(_ context.Context, id string, ids []string) error {
	return w.call("AddAssignees", id, strings.Join(ids, ","))
}

func (w *fakeWriter) RemoveAssignees(_ context.Context, id string, ids []string) error {
	return w.call("RemoveAssignees", id, strings.Join(ids, ","))
}

func (w *fakeWriter) AddLabels(_ context.Context, id string, ids []string) error {
	return w.call("AddLabels", id, strings.Join(ids, ","))
}

func (w *fakeWriter) RemoveLabels(_ context.Context, id string, ids []string) error {
	return w.call("RemoveLabels", id, strings.Join(ids, ","))
}

func (w *fakeWriter) AddComment(_ context.Context, subjectID, body string) (domain.Comment, error) {
	return w.comment, w.call("AddComment", subjectID, body)
}

func (w *fakeWriter) CloseIssue(_ context.Context, id string) error {
	return w.call("CloseIssue", id)
}

func (w *fakeWriter) ReopenIssue(_ context.Context, id string) error {
	return w.call("ReopenIssue", id)
}

func (w *fakeWriter) AddSubIssue(_ context.Context, parentID, childID string) error {
	return w.call("AddSubIssue", parentID, childID)
}

func (w *fakeWriter) UpdateIssueType(_ context.Context, issueID, typeID string) error {
	return w.call("UpdateIssueType", issueID, typeID)
}

func (w *fakeWriter) SetIssueFieldValue(_ context.Context, in domain.IssueFieldValueInput) error {
	return w.call("SetIssueFieldValue", in.IssueID, in.FieldID, in.Value)
}

func (w *fakeWriter) UpdateIssueFieldValue(_ context.Context, in domain.IssueFieldValueInput) error {
	return w.call("UpdateIssueFieldValue", in.IssueID, in.FieldID, in.Value)
}

func (w *fakeWriter) AddProjectItem(_ context.Context, projectID, contentID string) (string, error) {
	return w.itemID, w.call("AddProjectItem", projectID, contentID)
}

func (w *fakeWriter) UpdateItemFieldValue(_ context.Context, in domain.ItemFieldValueInput) error {
	return w.call("UpdateItemFieldValue", in.ProjectID, in.ItemID, in.FieldID, fieldValue(in))
}

// fieldValue names the one value member an input carries.
func fieldValue(in domain.ItemFieldValueInput) string {
	switch {
	case in.SingleSelectID != nil:
		return "option=" + *in.SingleSelectID
	case in.IterationID != nil:
		return "iteration=" + *in.IterationID
	case in.Number != nil:
		return "number=" + strconv.FormatFloat(*in.Number, 'f', -1, 64)
	case in.Date != nil:
		return "date=" + in.Date.Format(domain.DateLayout)
	case in.Text != nil:
		return "text=" + *in.Text
	}
	return "empty"
}

func (w *fakeWriter) UnarchiveItem(_ context.Context, projectID, itemID string) error {
	return w.call("UnarchiveItem", projectID, itemID)
}

func (w *fakeWriter) CreateStatusUpdate(_ context.Context, in domain.StatusUpdateInput) (string, error) {
	w.updates = append(w.updates, in)
	return w.updateID, w.call("CreateStatusUpdate", in.ProjectID, in.Status)
}

func (w *fakeWriter) CreateProjectField(_ context.Context, in domain.CreateFieldInput) (domain.Field, error) {
	return w.field, w.call("CreateProjectField", in.ProjectID, in.Name, string(in.DataType))
}

func (w *fakeWriter) CreateLabel(_ context.Context, in domain.CreateLabelInput) (domain.Label, error) {
	return w.label, w.call("CreateLabel", in.RepositoryID, in.Name)
}

func (w *fakeWriter) CreateMilestone(_ context.Context, in domain.CreateMilestoneInput) (domain.Milestone, error) {
	return w.milestone, w.call("CreateMilestone", in.Owner+"/"+in.Repo, in.Title)
}

func (w *fakeWriter) CopyProject(_ context.Context, in domain.CopyProjectInput) (domain.Project, error) {
	return w.project, w.call("CopyProject", in.SourceProjectID, in.Title)
}

func (w *fakeWriter) LinkProjectToRepository(_ context.Context, projectID, repositoryID string) error {
	return w.call("LinkProjectToRepository", projectID, repositoryID)
}
