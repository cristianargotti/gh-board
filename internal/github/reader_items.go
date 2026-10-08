package github

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// itemPageSize is the number of items read per request, the most the API
// gives. Each item carries up to 50 field values, 30 labels, 20 assignees
// and 30 issue field values, which keeps a page well under the node
// limit of the API.
const itemPageSize = 100

// Archived states of the items connection.
const (
	stateArchived    = "ARCHIVED"
	stateNotArchived = "NOT_ARCHIVED"
)

// ListItems returns one page of items with their field values. Archived is
// pushed down to the API, and so is the selection: the summary selection
// leaves the body, the milestone, the parent and the issue field member
// of the field values out of the document. State, Assignee, Label and
// Type apply on the page because they need no configuration; Status,
// Lane, Epic, Sprint, Overdue, Blocked and Triage are the caller's, who
// holds board.yml. Pull requests, draft issues and redacted items are
// skipped because the domain reads issues; TotalCount stays the count of
// the API. All reads every page.
func (a *Adapter) ListItems(ctx context.Context, project domain.Project, opts domain.ListOptions) (domain.ItemPage, error) {
	query := itemQuery{
		project: project.NodeID, first: pageSize(opts.Limit), after: opts.Cursor, filter: opts.Filter,
		full: opts.Selection != domain.SelectionSummary,
	}
	if !opts.All {
		return a.listPage(ctx, query)
	}
	all := []domain.Item{}
	skipped := 0
	for {
		page, err := a.listPage(ctx, query)
		if err != nil {
			return domain.ItemPage{}, err
		}
		all = append(all, page.Items...)
		skipped += page.Skipped
		if !page.HasNext {
			return domain.ItemPage{Items: all, TotalCount: page.TotalCount, Skipped: skipped}, nil
		}
		query.after = page.NextCursor
	}
}

type itemQuery struct {
	project string
	first   int
	after   string
	filter  domain.ItemFilter
	full    bool
}

// pageSize caps a page at the adapter size; a smaller limit is honored.
func pageSize(limit int) int {
	if limit > 0 && limit < itemPageSize {
		return limit
	}
	return itemPageSize
}

func (a *Adapter) listPage(ctx context.Context, q itemQuery) (domain.ItemPage, error) {
	archived := stateNotArchived
	if q.filter.Archived {
		archived = stateArchived
	}
	vars := pageVariables(map[string]any{"project": q.project, "archived": []string{archived}, varFull: q.full}, q.first, q.after)
	var out struct {
		Node *struct {
			Items *rawItemConnection `json:"items"`
		} `json:"node"`
	}
	if err := a.run(ctx, "list_items", vars, &out); err != nil {
		return domain.ItemPage{}, err
	}
	if out.Node == nil || out.Node.Items == nil {
		return domain.ItemPage{}, notFound("list_items", "project "+q.project)
	}
	conn := out.Node.Items
	page := domain.ItemPage{Items: []domain.Item{}, TotalCount: conn.TotalCount, HasNext: conn.PageInfo.HasNextPage}
	if page.HasNext {
		page.NextCursor = conn.PageInfo.EndCursor
	}
	for _, raw := range conn.Nodes {
		it, ok := toItem(raw)
		switch {
		case !ok:
			page.Skipped++
		case matches(it, q.filter):
			page.Items = append(page.Items, it)
		}
	}
	return page, nil
}

// matches applies the filters that need no board.yml: issue state,
// assignee login, label name and issue type name, all ignoring case.
func matches(it domain.Item, f domain.ItemFilter) bool {
	if f.State != "" && it.Issue.State != f.State {
		return false
	}
	if f.Assignee != "" && !it.AssignedTo(f.Assignee) {
		return false
	}
	if f.Label != "" && !it.HasLabel(f.Label) {
		return false
	}
	if f.Type != "" && (it.Issue.Type == nil || !strings.EqualFold(it.Issue.Type.Name, f.Type)) {
		return false
	}
	return true
}

// pageVariables builds the variables of a paginated document. The cursor
// is omitted on the first page because GitHub rejects an empty string.
func pageVariables(base map[string]any, first int, after string) map[string]any {
	vars := make(map[string]any, len(base)+2)
	for k, v := range base {
		vars[k] = v
	}
	vars["first"] = first
	if after != "" {
		vars["after"] = after
	}
	return vars
}

// forEachPage calls fetch with the cursor of every page until the last.
// A page that repeats its cursor ends the loop, so a broken server cannot
// spin the kit forever.
func forEachPage(fetch func(after string) (pageInfo, error)) error {
	after := ""
	for {
		info, err := fetch(after)
		if err != nil {
			return err
		}
		if !info.HasNextPage || info.EndCursor == "" || info.EndCursor == after {
			return nil
		}
		after = info.EndCursor
	}
}
