package commands

import (
	"context"
	"strconv"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func (f *writeFake) UpdateItemFieldValue(_ context.Context, in domain.ItemFieldValueInput) error {
	f.inputs = append(f.inputs, in)
	if err := f.writeErrorFor("setField"); err != nil {
		return err
	}
	for id, it := range f.items {
		if it.ProjectItemID != in.ItemID {
			continue
		}
		for _, field := range f.project.Fields {
			if field.ID == in.FieldID {
				it.Values[field.Name] = domain.FieldValue{Field: field.Name, Value: writeInputValue(field, in)}
			}
		}
		f.items[id] = it
	}
	return nil
}

func writeInputValue(field domain.Field, in domain.ItemFieldValueInput) string {
	switch {
	case in.Text != nil:
		return *in.Text
	case in.Number != nil:
		return strconv.FormatFloat(*in.Number, 'f', -1, 64)
	case in.Date != nil:
		return in.Date.Format(time.DateOnly)
	case in.SingleSelectID != nil:
		for _, option := range field.Options {
			if option.ID == *in.SingleSelectID {
				return option.Name
			}
		}
	case in.IterationID != nil:
		for _, iteration := range field.Iterations {
			if iteration.ID == *in.IterationID {
				return iteration.Title
			}
		}
	}
	return ""
}

func (f *writeFake) CreateIssue(_ context.Context, in domain.CreateIssueInput) (domain.Issue, error) {
	f.created = in
	if err := f.writeErrorFor("create"); err != nil {
		return domain.Issue{}, err
	}
	it := writeItem(4)
	it.ProjectItemID, it.Issue.Title, it.Issue.Body = "", in.Title, in.Body
	it.Values, it.IssueFields = map[string]domain.FieldValue{}, nil
	f.items[it.Issue.NodeID] = it
	return it.Issue, nil
}

func (f *writeFake) AddProjectItem(_ context.Context, _, id string) (string, error) {
	if err := f.writeErrorFor("addProject"); err != nil {
		return "", err
	}
	it := f.items[id]
	it.ProjectItemID = "PVTI_new"
	f.items[id] = it
	return it.ProjectItemID, nil
}

func (f *writeFake) UpdateIssue(_ context.Context, in domain.UpdateIssueInput) (domain.Issue, error) {
	if err := f.writeErrorFor("updateIssue"); err != nil {
		return domain.Issue{}, err
	}
	it := f.items[in.IssueID]
	if in.MilestoneID != nil {
		it.Issue.Milestone = &domain.Milestone{ID: *in.MilestoneID, Title: "Release"}
	}
	f.items[in.IssueID] = it
	return it.Issue, nil
}

func (f *writeFake) AddAssignees(_ context.Context, id string, ids []string) error {
	if err := f.writeErrorFor("assign"); err != nil {
		return err
	}
	it := f.items[id]
	for _, userID := range ids {
		it.Issue.Assignees = append(it.Issue.Assignees, domain.User{ID: userID, Login: "alice"})
	}
	f.items[id] = it
	return nil
}

func (f *writeFake) RemoveAssignees(_ context.Context, id string, _ []string) error {
	if err := f.writeErrorFor("unassign"); err != nil {
		return err
	}
	it := f.items[id]
	it.Issue.Assignees = nil
	f.items[id] = it
	return nil
}

func (f *writeFake) AddLabels(_ context.Context, id string, ids []string) error {
	if err := f.writeErrorFor("addLabels"); err != nil {
		return err
	}
	it := f.items[id]
	for _, label := range f.labels {
		if domain.Contains(ids, label.ID) {
			it.Issue.Labels = append(it.Issue.Labels, label)
		}
	}
	f.items[id] = it
	return nil
}

func (f *writeFake) RemoveLabels(_ context.Context, id string, ids []string) error {
	if err := f.writeErrorFor("removeLabels"); err != nil {
		return err
	}
	it := f.items[id]
	labels := []domain.Label{}
	for _, label := range it.Issue.Labels {
		if !domain.Contains(ids, label.ID) {
			labels = append(labels, label)
		}
	}
	it.Issue.Labels = labels
	f.items[id] = it
	return nil
}

func (f *writeFake) AddComment(_ context.Context, _, body string) (domain.Comment, error) {
	return domain.Comment{ID: "C_new", Body: body}, f.writeErrorFor("comment")
}

func (f *writeFake) AddSubIssue(_ context.Context, parent, child string) error {
	if err := f.writeErrorFor("link"); err != nil {
		return err
	}
	it := f.items[child]
	it.Issue.Parent = &domain.ParentRef{NodeID: parent}
	f.items[child] = it
	return nil
}

func (f *writeFake) ReopenIssue(_ context.Context, id string) error {
	if err := f.writeErrorFor("reopen"); err != nil {
		return err
	}
	it := f.items[id]
	it.Issue.State = domain.IssueOpen
	f.items[id] = it
	return nil
}

func (f *writeFake) UnarchiveItem(_ context.Context, _, itemID string) error {
	if err := f.writeErrorFor("restore"); err != nil {
		return err
	}
	for id, it := range f.items {
		if it.ProjectItemID == itemID {
			it.Archived = false
			f.items[id] = it
		}
	}
	return nil
}

func (f *writeFake) SetIssueFieldValue(_ context.Context, in domain.IssueFieldValueInput) error {
	return f.writeIssueDate("setIssueField", in)
}

func (f *writeFake) UpdateIssueFieldValue(_ context.Context, in domain.IssueFieldValueInput) error {
	return f.writeIssueDate("updateIssueField", in)
}

func (f *writeFake) writeIssueDate(call string, in domain.IssueFieldValueInput) error {
	if err := f.writeErrorFor(call); err != nil {
		return err
	}
	it := f.items[in.IssueID]
	fields := []domain.IssueFieldValue{}
	for _, field := range it.IssueFields {
		if field.FieldID != in.FieldID {
			fields = append(fields, field)
		}
	}
	for name, id := range f.fields {
		if id == in.FieldID {
			fields = append(fields, domain.IssueFieldValue{FieldID: id, Name: name, Value: in.Value})
		}
	}
	it.IssueFields = fields
	f.items[in.IssueID] = it
	return nil
}
