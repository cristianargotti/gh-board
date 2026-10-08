package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// addAssignees assigns the logins of After that are not assigned yet.
func (e *executor) addAssignees(ctx context.Context, step domain.Step) (Outcome, error) {
	issueID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	it, _ := e.item(step)
	logins := difference(SplitSet(step.After), SplitSet(domain.EncodeAssignees(it.Issue.Assignees)))
	if len(logins) == 0 {
		return skipped("", "already assigned")
	}
	ids := make([]string, 0, len(logins))
	for _, login := range logins {
		id, err := e.userID(ctx, login)
		if err != nil {
			return Outcome{}, err
		}
		ids = append(ids, id)
	}
	return finish(e.ports.Writer.AddAssignees(ctx, issueID, ids))
}

// userID resolves a login: the viewer knows its own id, any other login
// needs the resolver the adapter gains at integration.
func (e *executor) userID(ctx context.Context, login string) (string, error) {
	if strings.EqualFold(login, e.viewer.Login) && e.viewer.ID != "" {
		return e.viewer.ID, nil
	}
	resolver, ok := e.ports.Reader.(UserResolver)
	if !ok {
		return "", fmt.Errorf("assignee %q: the adapter cannot resolve logins yet: %w", login, domain.ErrNotImplemented)
	}
	return resolver.UserID(ctx, login)
}

// removeAssignees unassigns the current assignees that After leaves out.
func (e *executor) removeAssignees(ctx context.Context, step domain.Step) (Outcome, error) {
	issueID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	it, _ := e.item(step)
	keep := SplitSet(step.After)
	var ids []string
	for _, u := range it.Issue.Assignees {
		if !containsFold(keep, u.Login) {
			ids = append(ids, u.ID)
		}
	}
	if len(ids) == 0 {
		return skipped("", "nothing to unassign")
	}
	return finish(e.ports.Writer.RemoveAssignees(ctx, issueID, ids))
}

// addLabels adds the labels of After the issue does not carry, resolved
// against the repository labels.
func (e *executor) addLabels(ctx context.Context, step domain.Step) (Outcome, error) {
	issueID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	it, _ := e.item(step)
	names := difference(SplitSet(step.After), SplitSet(domain.EncodeLabels(it.Issue.Labels)))
	if len(names) == 0 {
		return skipped("", "labels already present")
	}
	owner, repo := splitRepo(step.Target.Repository)
	labels, err := e.ports.Reader.RepositoryLabels(ctx, owner, repo)
	if err != nil {
		return Outcome{}, err
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		label, err := domain.ResolveLabel(labels, name)
		if err != nil {
			return Outcome{}, err
		}
		ids = append(ids, label.ID)
	}
	return finish(e.ports.Writer.AddLabels(ctx, issueID, ids))
}

// removeLabels removes the labels After leaves out.
func (e *executor) removeLabels(ctx context.Context, step domain.Step) (Outcome, error) {
	issueID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	it, _ := e.item(step)
	keep := SplitSet(step.After)
	var ids []string
	for _, l := range it.Issue.Labels {
		if !containsFold(keep, l.Name) {
			ids = append(ids, l.ID)
		}
	}
	if len(ids) == 0 {
		return skipped("", "nothing to remove")
	}
	return finish(e.ports.Writer.RemoveLabels(ctx, issueID, ids))
}
