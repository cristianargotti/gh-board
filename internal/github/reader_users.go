package github

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// userPageSize is the page of the assignable users connection.
const userPageSize = 100

// AssignableUsers lists the users who can be assigned on a repository,
// so that assign resolves a login before asking GitHub for an id.
func (a *Adapter) AssignableUsers(ctx context.Context, owner, repo string) ([]domain.User, error) {
	if owner == "" || repo == "" {
		return nil, usage("assignable_users: owner and name are required")
	}
	users := []domain.User{}
	base := map[string]any{varOwner: owner, varName: repo}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Repository *struct {
				AssignableUsers struct {
					PageInfo pageInfo  `json:"pageInfo"`
					Nodes    []rawUser `json:"nodes"`
				} `json:"assignableUsers"`
			} `json:"repository"`
		}
		if err := a.run(ctx, "assignable_users", pageVariables(base, userPageSize, after), &out); err != nil {
			return pageInfo{}, err
		}
		if out.Repository == nil {
			return pageInfo{}, notFound("assignable_users", "repository "+owner+"/"+repo)
		}
		for _, raw := range out.Repository.AssignableUsers.Nodes {
			users = append(users, domain.User{ID: raw.ID, Login: raw.Login})
		}
		return out.Repository.AssignableUsers.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}
