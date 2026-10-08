package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Node kinds a reference may resolve to.
const (
	nodeProjectItem = "ProjectV2Item"
	nodeIssue       = "Issue"
)

// rawItemNode is the ItemByNode answer: a project item, or an issue with
// its items on every project.
type rawItemNode struct {
	rawItem
	TypeName     string       `json:"__typename"`
	ProjectItems *rawItemList `json:"projectItems"`
}

// GetItem resolves a reference to exactly one item, fresh from the API.
// A bare number resolves among the active items of the board and must
// match exactly one issue across the linked repositories (section 6.6).
func (a *Adapter) GetItem(ctx context.Context, project domain.Project, ref domain.Reference) (domain.Item, error) {
	switch ref.Kind {
	case domain.RefNodeID:
		return a.itemByNode(ctx, project, ref.NodeID)
	case domain.RefFull, domain.RefURL:
		return a.itemByNumber(ctx, project, ref.Owner, ref.Repo, ref.Number)
	case domain.RefNumber:
		return a.itemByBareNumber(ctx, project, ref.Number)
	default:
		return domain.Item{}, usage("reference %q has no kind", ref.Raw)
	}
}

func (a *Adapter) itemByNode(ctx context.Context, project domain.Project, id string) (domain.Item, error) {
	var out struct {
		Node *rawItemNode `json:"node"`
	}
	if err := a.run(ctx, "item_by_node", map[string]any{varID: id}, &out); err != nil {
		return domain.Item{}, err
	}
	if out.Node == nil {
		return domain.Item{}, notFound("item_by_node", "node "+id)
	}
	switch out.Node.TypeName {
	case nodeProjectItem:
		return pickItem([]rawItem{out.Node.rawItem}, project, id)
	case nodeIssue:
		if out.Node.ProjectItems == nil {
			return domain.Item{}, notFound("item_by_node", "issue "+id+" has no project items")
		}
		return pickItem(out.Node.ProjectItems.Nodes, project, id)
	default:
		return domain.Item{}, notFound("item_by_node", fmt.Sprintf("node %s is a %s, not an issue or a project item", id, out.Node.TypeName))
	}
}

// pickItem keeps the issue item that belongs to the board.
func pickItem(items []rawItem, project domain.Project, ref string) (domain.Item, error) {
	for _, raw := range items {
		if raw.Project.ID != project.NodeID {
			continue
		}
		if it, ok := toItem(raw); ok {
			return it, nil
		}
		return domain.Item{}, notFound("item", ref+" is not an issue")
	}
	return domain.Item{}, notFound("item", ref+" is not on project "+project.Ref.String())
}

func (a *Adapter) itemByNumber(ctx context.Context, project domain.Project, owner, repo string, number int) (domain.Item, error) {
	var out struct {
		Repository *struct {
			Issue *struct {
				ID           string       `json:"id"`
				ProjectItems *rawItemList `json:"projectItems"`
			} `json:"issue"`
		} `json:"repository"`
	}
	vars := map[string]any{varOwner: owner, varName: repo, varNumber: number}
	if err := a.run(ctx, "issue_by_number", vars, &out); err != nil {
		return domain.Item{}, err
	}
	ref := fmt.Sprintf("%s/%s#%d", owner, repo, number)
	if out.Repository == nil || out.Repository.Issue == nil || out.Repository.Issue.ProjectItems == nil {
		return domain.Item{}, notFound("issue_by_number", ref)
	}
	return pickItem(out.Repository.Issue.ProjectItems.Nodes, project, ref)
}

// itemByBareNumber finds the number among the summaries of the board,
// which read faster, and then reads the one item in full and fresh.
func (a *Adapter) itemByBareNumber(ctx context.Context, project domain.Project, number int) (domain.Item, error) {
	page, err := a.ListItems(ctx, project, domain.ListOptions{All: true, Selection: domain.SelectionSummary})
	if err != nil {
		return domain.Item{}, err
	}
	var found []domain.Item
	for _, it := range page.Items {
		if it.Issue.Number == number {
			found = append(found, it)
		}
	}
	switch len(found) {
	case 0:
		return domain.Item{}, notFound("list_items", fmt.Sprintf("#%d on project %s", number, project.Ref))
	case 1:
		return a.itemByNode(ctx, project, found[0].ProjectItemID)
	default:
		refs := make([]string, 0, len(found))
		for _, it := range found {
			refs = append(refs, it.Issue.Ref())
		}
		return domain.Item{}, ambiguous("list_items", fmt.Sprintf("#%d matches %s: use owner/repo#n", number, strings.Join(refs, ", ")))
	}
}
