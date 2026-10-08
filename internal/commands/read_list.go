package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readListDefaultLimit is the page size without --all.
const readListDefaultLimit = 50

type readListFlags struct {
	filter domain.ItemFilter
	all    bool
	limit  int
}

// readListFilter echoes the active filters in --json.
type readListFilter struct {
	Status   string `json:"status,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Lane     string `json:"lane,omitempty"`
	Epic     string `json:"epic,omitempty"`
	Sprint   string `json:"sprint,omitempty"`
	Label    string `json:"label,omitempty"`
	Type     string `json:"type,omitempty"`
	Overdue  bool   `json:"overdue"`
	Blocked  bool   `json:"blocked"`
	Triage   bool   `json:"triage"`
}

// readListPayload is the --json form of list.
type readListPayload struct {
	GeneratedAt string         `json:"generated_at"`
	Filter      readListFilter `json:"filter"`
	Items       []readListItem `json:"items"`
	Shown       int            `json:"shown"`
	Total       int            `json:"total,omitempty"`
	More        bool           `json:"more"`
	// Omitted counts the board items read that are not issues.
	Omitted int `json:"omitted"`
}

type readListItem struct {
	domain.ItemSummary
	State  string   `json:"state"`
	Type   string   `json:"type,omitempty"`
	Labels []string `json:"labels"`
}

func readListCommand(deps *Deps) *cobra.Command {
	var flags readListFlags
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List items with the filters of section 7; --all reads every page",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readList(cmd.Context(), deps, flags) },
	}
	f := cmd.Flags()
	f.StringVar(&flags.filter.Status, "status", "", "status option name")
	f.StringVar(&flags.filter.Assignee, "assignee", "", "assignee login")
	f.StringVar(&flags.filter.Lane, "lane", "", "lane option name (capabilities.lane)")
	f.StringVar(&flags.filter.Epic, "epic", "", "epic value (capabilities.epic.field)")
	f.StringVar(&flags.filter.Sprint, "sprint", "", "iteration title, current or next (capabilities.sprint)")
	f.StringVar(&flags.filter.Label, "label", "", "label name")
	f.StringVar(&flags.filter.Type, "type", "", "issue type name")
	f.BoolVar(&flags.filter.Overdue, "overdue", false, "only items past their target date (capabilities.dates)")
	f.BoolVar(&flags.filter.Blocked, "blocked", false, "only blocked items")
	f.BoolVar(&flags.filter.Triage, "triage", false, "only undecided triage entries (capabilities.triage)")
	f.BoolVar(&flags.all, "all", false, "read every page")
	f.IntVar(&flags.limit, "limit", readListDefaultLimit, "items per page without --all")
	return cmd
}

func readList(ctx context.Context, deps *Deps, flags readListFlags) error {
	if flags.limit <= 0 {
		return domain.Errorf(domain.ExitUsage, "--limit must be positive")
	}
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	if err := readListCheck(s, &flags.filter); err != nil {
		return err
	}
	items, page, err := readListPages(ctx, s, flags)
	if err != nil {
		return err
	}
	loc := domain.LocationOf(s.cfg)
	payload := readListPayload{GeneratedAt: readStamp(s.now), Filter: readListFilterOf(flags.filter), Items: []readListItem{}}
	for _, it := range items {
		if readMatches(s.cfg, it, flags.filter, s.now, loc) {
			payload.Items = append(payload.Items, readListItemOf(s.cfg, it))
		}
	}
	payload.Shown = len(payload.Items)
	payload.Omitted = s.skipped
	payload.More = !flags.all && page.HasNext
	if payload.More {
		payload.Total = page.TotalCount
	}
	doc := readListDocument(payload, s.cfg)
	doc.Data = payload
	return readRender(deps, doc)
}

// readListCheck refuses filters whose capability the board does not map
// and resolves the sprint aliases current and next.
func readListCheck(s *readSession, f *domain.ItemFilter) error {
	caps := domain.CapabilitiesOf(s.cfg)
	needs := []struct {
		on   bool
		ok   bool
		flag string
	}{
		{f.Lane != "", caps.Lane != nil, "--lane"},
		{f.Epic != "", caps.Epic != nil && caps.Epic.Field != "", "--epic"},
		{f.Sprint != "", caps.Sprint != nil, "--sprint"},
		{f.Overdue, caps.Dates != nil, "--overdue"},
		{f.Triage, caps.Triage != nil, "--triage"},
	}
	for _, n := range needs {
		if n.on && !n.ok {
			return domain.Errorf(domain.ExitUsage, "%s: %s", n.flag, readUnavailable)
		}
	}
	if f.Sprint == "current" || f.Sprint == "next" {
		_, it, reason := readResolveIteration(s.cfg, s.project, domain.Today(s.now, domain.LocationOf(s.cfg)), f.Sprint)
		if reason != "" {
			return domain.Errorf(domain.ExitUsage, "--sprint %s: %s", f.Sprint, reason)
		}
		f.Sprint = it.Title
	}
	return nil
}

// readListPages reads one page, or every page with --all. The filter is
// handed to the adapter so it can push it down; the client applies it
// again, which keeps the result identical either way.
func readListPages(ctx context.Context, s *readSession, flags readListFlags) ([]domain.Item, domain.ItemPage, error) {
	var items []domain.Item
	cursor := ""
	seen := map[string]bool{}
	for {
		opts := domain.ListOptions{All: flags.all, Filter: flags.filter, Cursor: cursor, Selection: domain.SelectionSummary}
		if !flags.all {
			opts.Limit = flags.limit
		}
		page, err := s.deps.Reader.ListItems(ctx, s.project, opts)
		if err != nil {
			return nil, page, readAPI(err)
		}
		items = append(items, page.Items...)
		s.skipped += page.Skipped
		if !flags.all || !page.HasNext {
			return items, page, nil
		}
		if page.NextCursor == "" || seen[page.NextCursor] {
			return nil, page, domain.Errorf(domain.ExitAPI, "item pagination did not advance")
		}
		cursor, seen[page.NextCursor] = page.NextCursor, true
	}
}

func readListFilterOf(f domain.ItemFilter) readListFilter {
	return readListFilter{
		Status: f.Status, Assignee: f.Assignee, Lane: f.Lane, Epic: f.Epic, Sprint: f.Sprint, Label: f.Label,
		Type: f.Type, Overdue: f.Overdue, Blocked: f.Blocked, Triage: f.Triage,
	}
}

func readListItemOf(cfg *domain.Config, it domain.Item) readListItem {
	sum := domain.SummarizeItem(cfg, it)
	sum.Title = render.Title(sum.Title)
	out := readListItem{ItemSummary: sum, State: string(it.Issue.State), Labels: readLabelList(it.Issue.Labels)}
	if it.Issue.Type != nil {
		out.Type = it.Issue.Type.Name
	}
	return out
}

func readListDocument(p readListPayload, cfg *domain.Config) *render.Document {
	doc := render.NewDocument(readKeyItems)
	sec := doc.AddSection("")
	if len(p.Items) == 0 {
		sec.AddNote("no items match")
	}
	cols := readColumnsOf(cfg)
	header := []string{readColRef, readColState, readColStatus, readColAssignees, readColTitle, "LABELS"}
	optional := []struct {
		on   bool
		name string
	}{{cols.lane, readColLane}, {cols.epic, readColEpic}, {cols.sprint, readColSprint}, {cols.target, readColTarget}}
	for _, o := range optional {
		if o.on {
			header = append(header, o.name)
		}
	}
	t := sec.SetTable(header...)
	for _, it := range p.Items {
		row := []string{it.Ref, it.State, it.Status, readJoin(it.Assignees), it.Title, readJoin(it.Labels)}
		values := []string{it.Lane, it.Epic, it.Sprint, it.Target}
		for i, o := range optional {
			if o.on {
				row = append(row, values[i])
			}
		}
		t.AddRow(row...)
	}
	sec.AddNote(readCount(p.Shown, "item", "items") + " shown")
	if note := readSkippedNote(p.Omitted); note != "" {
		sec.AddNote(note)
	}
	if p.More {
		sec.AddNote(readListMoreNote(p))
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, p.GeneratedAt)
	return doc
}

func readListMoreNote(p readListPayload) string {
	if p.Total > 0 {
		return readCount(p.Total, "item", "items") + " on the board before filtering, use --all for every page"
	}
	return "more items available, use --all for every page"
}
