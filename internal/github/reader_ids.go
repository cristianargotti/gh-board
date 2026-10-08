package github

import (
	"context"
	"fmt"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// IssueByID reads one issue by node id, fresh from the API: new re-reads
// the issue it created before adding it to the board.
func (a *Adapter) IssueByID(ctx context.Context, id string) (domain.Issue, error) {
	if id == "" {
		return domain.Issue{}, usage("issue_by_id: an issue id is required")
	}
	var out struct {
		Node *rawIssue `json:"node"`
	}
	if err := a.run(ctx, "issue_by_id", map[string]any{varID: id}, &out); err != nil {
		return domain.Issue{}, err
	}
	if out.Node == nil || out.Node.TypeName != contentIssue {
		return domain.Issue{}, notFound("issue_by_id", fmt.Sprintf("node %s is not an issue", id))
	}
	return toIssue(*out.Node), nil
}

// UserID resolves a login to a user node id, for an assignee who is not
// the viewer and not yet on the issue.
func (a *Adapter) UserID(ctx context.Context, login string) (string, error) {
	return a.loginID(ctx, "user_id", "user", login)
}

// OwnerID resolves an organization or user login to its node id, the
// owner that receives a project copy.
func (a *Adapter) OwnerID(ctx context.Context, login string) (string, error) {
	return a.loginID(ctx, "owner_id", "repositoryOwner", login)
}

// loginID runs a document whose single root field answers the node of a
// login. GitHub reports a missing login as NOT_FOUND on that path.
func (a *Adapter) loginID(ctx context.Context, doc, root, login string) (string, error) {
	if login == "" {
		return "", usage("%s: a login is required", doc)
	}
	var out map[string]*rawUser
	if err := a.run(ctx, doc, map[string]any{varLogin: login}, &out); err != nil {
		return "", err
	}
	if node := out[root]; node != nil && node.ID != "" {
		return node.ID, nil
	}
	return "", notFound(doc, "login "+login)
}

// RepositoryID resolves owner/name to the repository node id, for a
// configured repository the project does not link.
func (a *Adapter) RepositoryID(ctx context.Context, owner, repo string) (string, error) {
	if owner == "" || repo == "" {
		return "", usage("repository_id: owner and name are required")
	}
	var out struct {
		Repository *struct {
			ID string `json:"id"`
		} `json:"repository"`
	}
	if err := a.run(ctx, "repository_id", map[string]any{varOwner: owner, varName: repo}, &out); err != nil {
		return "", err
	}
	if out.Repository == nil || out.Repository.ID == "" {
		return "", notFound("repository_id", "repository "+owner+"/"+repo)
	}
	return out.Repository.ID, nil
}
