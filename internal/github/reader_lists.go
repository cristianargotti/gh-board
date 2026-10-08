package github

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Page sizes of the metadata connections.
const (
	labelPageSize     = 100
	milestonePageSize = 100
	issueTypePageSize = 50
	commentPageSize   = 100
)

// RepositoryLabels lists the labels of a repository, by name.
func (a *Adapter) RepositoryLabels(ctx context.Context, owner, repo string) ([]domain.Label, error) {
	labels := []domain.Label{}
	base := map[string]any{varOwner: owner, varName: repo}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Repository *struct {
				Labels struct {
					PageInfo pageInfo   `json:"pageInfo"`
					Nodes    []rawLabel `json:"nodes"`
				} `json:"labels"`
			} `json:"repository"`
		}
		if err := a.run(ctx, "repository_labels", pageVariables(base, labelPageSize, after), &out); err != nil {
			return pageInfo{}, err
		}
		if out.Repository == nil {
			return pageInfo{}, notFound("repository_labels", "repository "+owner+"/"+repo)
		}
		for _, raw := range out.Repository.Labels.Nodes {
			labels = append(labels, toLabel(raw))
		}
		return out.Repository.Labels.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return labels, nil
}

// RepositoryMilestones lists the open milestones of a repository, by due
// date.
func (a *Adapter) RepositoryMilestones(ctx context.Context, owner, repo string) ([]domain.Milestone, error) {
	milestones := []domain.Milestone{}
	base := map[string]any{varOwner: owner, varName: repo}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Repository *struct {
				Milestones struct {
					PageInfo pageInfo       `json:"pageInfo"`
					Nodes    []rawMilestone `json:"nodes"`
				} `json:"milestones"`
			} `json:"repository"`
		}
		if err := a.run(ctx, "repository_milestones", pageVariables(base, milestonePageSize, after), &out); err != nil {
			return pageInfo{}, err
		}
		if out.Repository == nil {
			return pageInfo{}, notFound("repository_milestones", "repository "+owner+"/"+repo)
		}
		for _, raw := range out.Repository.Milestones.Nodes {
			milestones = append(milestones, toMilestone(raw))
		}
		return out.Repository.Milestones.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return milestones, nil
}

// IssueTypes lists the enabled issue types of an organization. A user
// owner has none, so the NOT_FOUND of the organization path answers an
// empty list instead of an error.
func (a *Adapter) IssueTypes(ctx context.Context, owner string) ([]domain.IssueType, error) {
	types := []domain.IssueType{}
	base := map[string]any{varOwner: owner}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Organization *struct {
				IssueTypes struct {
					PageInfo pageInfo       `json:"pageInfo"`
					Nodes    []rawIssueType `json:"nodes"`
				} `json:"issueTypes"`
			} `json:"organization"`
		}
		err := a.run(ctx, "issue_types", pageVariables(base, issueTypePageSize, after), &out)
		if err != nil && !tolerated(err, "organization") {
			return pageInfo{}, err
		}
		if out.Organization == nil {
			return pageInfo{}, nil
		}
		for _, raw := range out.Organization.IssueTypes.Nodes {
			if raw.IsEnabled {
				types = append(types, domain.IssueType{ID: raw.ID, Name: raw.Name})
			}
		}
		return out.Organization.IssueTypes.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return types, nil
}

// IssueComments reads the comments of an issue, oldest first, which the
// first_response metric of the digest walks.
func (a *Adapter) IssueComments(ctx context.Context, issueID string) ([]domain.Comment, error) {
	comments := []domain.Comment{}
	base := map[string]any{varID: issueID}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Node *struct {
				Comments *struct {
					PageInfo pageInfo     `json:"pageInfo"`
					Nodes    []rawComment `json:"nodes"`
				} `json:"comments"`
			} `json:"node"`
		}
		if err := a.run(ctx, "issue_comments", pageVariables(base, commentPageSize, after), &out); err != nil {
			return pageInfo{}, err
		}
		if out.Node == nil || out.Node.Comments == nil {
			return pageInfo{}, notFound("issue_comments", "issue "+issueID)
		}
		for _, raw := range out.Node.Comments.Nodes {
			comments = append(comments, toComment(raw))
		}
		return out.Node.Comments.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return comments, nil
}
