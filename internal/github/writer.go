package github

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// setString adds a variable only when it has a value, so GitHub keeps the
// attribute untouched instead of receiving an empty string.
func setString(vars map[string]any, key, value string) {
	if value != "" {
		vars[key] = value
	}
}

// setList adds a list variable only when it has entries.
func setList(vars map[string]any, key string, values []string) {
	if len(values) > 0 {
		vars[key] = values
	}
}

// issueResult is the payload of the mutations that return IssueFields.
type issueResult struct {
	Issue rawIssue `json:"issue"`
}

// CreateIssue runs createIssue.
func (a *Adapter) CreateIssue(ctx context.Context, in domain.CreateIssueInput) (domain.Issue, error) {
	if in.RepositoryID == "" || in.Title == "" {
		return domain.Issue{}, usage("create_issue: repository id and title are required")
	}
	vars := map[string]any{varRepositoryID: in.RepositoryID, varTitle: in.Title}
	setString(vars, varBody, in.Body)
	setString(vars, "issueTypeId", in.IssueTypeID)
	setString(vars, "milestoneId", in.MilestoneID)
	setList(vars, "assigneeIds", in.AssigneeIDs)
	setList(vars, "labelIds", in.LabelIDs)
	var out struct {
		CreateIssue issueResult `json:"createIssue"`
	}
	if err := a.run(ctx, "create_issue", vars, &out); err != nil {
		return domain.Issue{}, err
	}
	return toIssue(out.CreateIssue.Issue), nil
}

// UpdateIssue runs updateIssue for title, body and milestone only. A nil
// member stays untouched; the milestone can be set, never cleared.
func (a *Adapter) UpdateIssue(ctx context.Context, in domain.UpdateIssueInput) (domain.Issue, error) {
	vars := map[string]any{varID: in.IssueID}
	if in.Title != nil {
		vars[varTitle] = *in.Title
	}
	if in.Body != nil {
		vars[varBody] = *in.Body
	}
	if in.MilestoneID != nil {
		if *in.MilestoneID == "" {
			return domain.Issue{}, usage("update_issue: the milestone can be set, not cleared")
		}
		vars["milestoneId"] = *in.MilestoneID
	}
	if len(vars) == 1 {
		return domain.Issue{}, usage("update_issue: nothing to update on %s", in.IssueID)
	}
	var out struct {
		UpdateIssue issueResult `json:"updateIssue"`
	}
	if err := a.run(ctx, "update_issue", vars, &out); err != nil {
		return domain.Issue{}, err
	}
	return toIssue(out.UpdateIssue.Issue), nil
}

// AddAssignees runs addAssigneesToAssignable.
func (a *Adapter) AddAssignees(ctx context.Context, assignableID string, userIDs []string) error {
	return a.changeSet(ctx, "add_assignees", "assignableId", assignableID, "assigneeIds", userIDs)
}

// RemoveAssignees runs removeAssigneesFromAssignable.
func (a *Adapter) RemoveAssignees(ctx context.Context, assignableID string, userIDs []string) error {
	return a.changeSet(ctx, "remove_assignees", "assignableId", assignableID, "assigneeIds", userIDs)
}

// AddLabels runs addLabelsToLabelable.
func (a *Adapter) AddLabels(ctx context.Context, labelableID string, labelIDs []string) error {
	return a.changeSet(ctx, "add_labels", "labelableId", labelableID, "labelIds", labelIDs)
}

// RemoveLabels runs removeLabelsFromLabelable.
func (a *Adapter) RemoveLabels(ctx context.Context, labelableID string, labelIDs []string) error {
	return a.changeSet(ctx, "remove_labels", "labelableId", labelableID, "labelIds", labelIDs)
}

// changeSet runs one of the four set mutations; an empty id list is a
// usage error because GitHub would accept it and change nothing.
func (a *Adapter) changeSet(ctx context.Context, doc, subjectKey, subjectID, listKey string, ids []string) error {
	if subjectID == "" || len(ids) == 0 {
		return usage("%s: a subject id and at least one id are required", doc)
	}
	var out struct{}
	return a.run(ctx, doc, map[string]any{subjectKey: subjectID, listKey: ids}, &out)
}

// AddComment runs addComment and returns the created comment.
func (a *Adapter) AddComment(ctx context.Context, subjectID, body string) (domain.Comment, error) {
	if subjectID == "" || body == "" {
		return domain.Comment{}, usage("add_comment: a subject id and a body are required")
	}
	var out struct {
		AddComment struct {
			CommentEdge struct {
				Node rawComment `json:"node"`
			} `json:"commentEdge"`
		} `json:"addComment"`
	}
	if err := a.run(ctx, "add_comment", map[string]any{"subjectId": subjectID, varBody: body}, &out); err != nil {
		return domain.Comment{}, err
	}
	return toComment(out.AddComment.CommentEdge.Node), nil
}

// CloseIssue runs closeIssue.
func (a *Adapter) CloseIssue(ctx context.Context, issueID string) error {
	return a.issueOnly(ctx, "close_issue", issueID)
}

// ReopenIssue runs reopenIssue.
func (a *Adapter) ReopenIssue(ctx context.Context, issueID string) error {
	return a.issueOnly(ctx, "reopen_issue", issueID)
}

func (a *Adapter) issueOnly(ctx context.Context, doc, issueID string) error {
	if issueID == "" {
		return usage("%s: an issue id is required", doc)
	}
	var out struct{}
	return a.run(ctx, doc, map[string]any{varIssueID: issueID}, &out)
}

// AddSubIssue runs addSubIssue, linking child under parent.
func (a *Adapter) AddSubIssue(ctx context.Context, parentID, childID string) error {
	if parentID == "" || childID == "" {
		return usage("add_sub_issue: parent and child ids are required")
	}
	var out struct{}
	return a.run(ctx, "add_sub_issue", map[string]any{varIssueID: parentID, "subIssueId": childID}, &out)
}

// UpdateIssueType runs updateIssueIssueType. An empty type would clear the
// type, which the kit never does.
func (a *Adapter) UpdateIssueType(ctx context.Context, issueID, issueTypeID string) error {
	if issueID == "" || issueTypeID == "" {
		return usage("update_issue_type: issue and type ids are required")
	}
	var out struct{}
	return a.run(ctx, "update_issue_type", map[string]any{varIssueID: issueID, "issueTypeId": issueTypeID}, &out)
}
