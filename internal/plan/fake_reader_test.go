package plan_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

// fakeClock returns a fixed instant.
type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

// fakeReader serves fixtures and records every call; fail injects an
// error per method name. It also implements the optional ports.
type fakeReader struct {
	project    domain.Project
	items      map[string]domain.Item
	viewer     domain.Viewer
	labels     []domain.Label
	milestones []domain.Milestone
	types      []domain.IssueType
	comments   map[string][]domain.Comment
	users      map[string]string
	fields     map[string]string
	updates    []plan.StatusUpdate
	issues     map[string]domain.Issue
	fail       map[string]error
	calls      []string
}

func (r *fakeReader) call(name string, args ...string) error {
	r.calls = append(r.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	return r.fail[name]
}

func (r *fakeReader) DiscoverProject(_ context.Context, ref domain.ProjectRef) (domain.Project, error) {
	if err := r.call("DiscoverProject", ref.String()); err != nil {
		return domain.Project{}, err
	}
	return r.project, nil
}

func (r *fakeReader) ListItems(_ context.Context, _ domain.Project, _ domain.ListOptions) (domain.ItemPage, error) {
	return domain.ItemPage{}, r.call("ListItems")
}

func (r *fakeReader) GetItem(_ context.Context, _ domain.Project, ref domain.Reference) (domain.Item, error) {
	key := ref.NodeID
	if key == "" {
		key = fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Number)
	}
	if err := r.call("GetItem", key); err != nil {
		return domain.Item{}, err
	}
	it, ok := r.items[key]
	if !ok {
		return domain.Item{}, fmt.Errorf("%s: %w", key, domain.ErrNotFound)
	}
	return it, nil
}

func (r *fakeReader) Viewer(context.Context) (domain.Viewer, error) {
	return r.viewer, r.call("Viewer")
}

func (r *fakeReader) ViewerPermission(_ context.Context, owner, repo string) (domain.Permission, error) {
	return domain.PermissionWrite, r.call("ViewerPermission", owner+"/"+repo)
}

func (r *fakeReader) RepositoryLabels(_ context.Context, owner, repo string) ([]domain.Label, error) {
	return r.labels, r.call("RepositoryLabels", owner+"/"+repo)
}

func (r *fakeReader) RepositoryMilestones(_ context.Context, owner, repo string) ([]domain.Milestone, error) {
	return r.milestones, r.call("RepositoryMilestones", owner+"/"+repo)
}

func (r *fakeReader) IssueTypes(_ context.Context, owner string) ([]domain.IssueType, error) {
	return r.types, r.call("IssueTypes", owner)
}

func (r *fakeReader) IssueComments(_ context.Context, issueID string) ([]domain.Comment, error) {
	return r.comments[issueID], r.call("IssueComments", issueID)
}

func (r *fakeReader) UserID(_ context.Context, login string) (string, error) {
	if err := r.call("UserID", login); err != nil {
		return "", err
	}
	id, ok := r.users[login]
	if !ok {
		return "", fmt.Errorf("user %s: %w", login, domain.ErrNotFound)
	}
	return id, nil
}

func (r *fakeReader) IssueFieldID(_ context.Context, owner, name string) (string, error) {
	if err := r.call("IssueFieldID", owner, name); err != nil {
		return "", err
	}
	id, ok := r.fields[name]
	if !ok {
		return "", fmt.Errorf("issue field %s: %w", name, domain.ErrNotFound)
	}
	return id, nil
}

func (r *fakeReader) IssueByID(_ context.Context, id string) (domain.Issue, error) {
	if err := r.call("IssueByID", id); err != nil {
		return domain.Issue{}, err
	}
	issue, ok := r.issues[id]
	if !ok {
		return domain.Issue{}, fmt.Errorf("%s: %w", id, domain.ErrNotFound)
	}
	return issue, nil
}

func (r *fakeReader) ListStatusUpdates(_ context.Context, projectID string) ([]plan.StatusUpdate, error) {
	return r.updates, r.call("ListStatusUpdates", projectID)
}

// bareReader hides the optional ports of the fake reader.
type bareReader struct{ domain.ProjectReader }
