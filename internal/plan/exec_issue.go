package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// closeIssue runs closeIssue on the target.
func (e *executor) closeIssue(ctx context.Context, step domain.Step) (Outcome, error) {
	id, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	return finish(e.ports.Writer.CloseIssue(ctx, id))
}

// reopenIssue runs reopenIssue on the target.
func (e *executor) reopenIssue(ctx context.Context, step domain.Step) (Outcome, error) {
	id, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	return finish(e.ports.Writer.ReopenIssue(ctx, id))
}

// updateIssue changes the title, the body or the milestone named in
// Field; After carries the new text or the milestone title.
func (e *executor) updateIssue(ctx context.Context, step domain.Step) (Outcome, error) {
	id, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	in := domain.UpdateIssueInput{IssueID: id}
	after := step.After
	switch step.Field {
	case domain.UpdateFieldTitle:
		in.Title = &after
	case domain.UpdateFieldBody:
		in.Body = &after
	case domain.UpdateFieldMilestone:
		milestoneID, err := e.milestoneID(ctx, step)
		if err != nil {
			return Outcome{}, err
		}
		in.MilestoneID = &milestoneID
	default:
		return Outcome{}, fmt.Errorf("update_issue field %q is not title, body or milestone: %w", step.Field, domain.ErrUsage)
	}
	_, err = e.ports.Writer.UpdateIssue(ctx, in)
	return finish(err)
}

// milestoneID resolves the milestone title of After in the repository of
// the target. Removing a milestone is not a kit operation.
func (e *executor) milestoneID(ctx context.Context, step domain.Step) (string, error) {
	if strings.TrimSpace(step.After) == "" {
		return "", fmt.Errorf("update_issue cannot remove a milestone: %w", domain.ErrUsage)
	}
	owner, repo := splitRepo(step.Target.Repository)
	milestones, err := e.ports.Reader.RepositoryMilestones(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	m, err := domain.ResolveMilestone(milestones, step.After)
	if err != nil {
		return "", err
	}
	return m.ID, nil
}

// setIssueType sets the organization issue type named in After.
func (e *executor) setIssueType(ctx context.Context, step domain.Step) (Outcome, error) {
	id, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	owner, _ := splitRepo(step.Target.Repository)
	types, err := e.ports.Reader.IssueTypes(ctx, owner)
	if err != nil {
		return Outcome{}, err
	}
	for _, t := range types {
		if strings.EqualFold(t.Name, step.After) {
			return finish(e.ports.Writer.UpdateIssueType(ctx, id, t.ID))
		}
	}
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.Name)
	}
	return Outcome{}, domain.NotFound("issue type", step.After, names)
}

// addSubIssue links the target under the parent named in After, a
// reference the reader resolves: owner/repo#n, a URL or a node id.
func (e *executor) addSubIssue(ctx context.Context, step domain.Step) (Outcome, error) {
	childID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	ref, err := domain.ParseReference(step.After)
	if err != nil {
		return Outcome{}, err
	}
	if ref.NeedsRepository() {
		owner, repo := splitRepo(step.Target.Repository)
		ref = ref.WithRepository(owner, repo)
	}
	parent, err := e.ports.Reader.GetItem(ctx, e.snap.Project, ref)
	if err != nil {
		return Outcome{}, fmt.Errorf("parent %s: %w", step.After, err)
	}
	return finish(e.ports.Writer.AddSubIssue(ctx, parent.Issue.NodeID, childID))
}

// unarchiveItem restores the project item of the target.
func (e *executor) unarchiveItem(ctx context.Context, step domain.Step) (Outcome, error) {
	itemID, err := e.itemID(step)
	if err != nil {
		return Outcome{}, err
	}
	return finish(e.ports.Writer.UnarchiveItem(ctx, e.projectID(), itemID))
}

// addProjectItem adds the target issue to the project, or skips it when
// the re-read found it on the board already.
func (e *executor) addProjectItem(ctx context.Context, step domain.Step) (Outcome, error) {
	contentID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	if it, ok := e.item(step); ok && it.ProjectItemID != "" {
		return skipped(it.ProjectItemID, "already on the board")
	}
	itemID, err := e.ports.Writer.AddProjectItem(ctx, e.projectID(), contentID)
	if err != nil {
		return Outcome{}, err
	}
	return created(itemID, "added to the board")
}

// itemID resolves the project item of a step: the one the step names,
// else the one the re-read found.
func (e *executor) itemID(step domain.Step) (string, error) {
	id, err := e.resolve(step.Target.ProjectItemID)
	if err != nil {
		return "", err
	}
	if id == "" {
		if it, ok := e.item(step); ok {
			id = it.ProjectItemID
		}
	}
	if id == "" {
		return "", fmt.Errorf("step %d (%s) names no project item: %w", step.Index, step.Operation, domain.ErrUsage)
	}
	return id, nil
}
