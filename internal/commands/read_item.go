package commands

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readItemPayload is the --json form of item: the issue, its board
// fields, parent, sub-issue progress, blockers and dates with their source.
type readItemPayload struct {
	GeneratedAt    string                 `json:"generated_at"`
	Ref            string                 `json:"ref"`
	NodeID         string                 `json:"node_id"`
	ProjectItemID  string                 `json:"project_item_id"`
	Archived       bool                   `json:"archived"`
	Title          string                 `json:"title"`
	State          string                 `json:"state"`
	URL            string                 `json:"url"`
	Type           string                 `json:"type,omitempty"`
	Labels         []string               `json:"labels"`
	Assignees      []string               `json:"assignees"`
	Milestone      *readItemMilestone     `json:"milestone,omitempty"`
	Parent         *readItemParent        `json:"parent,omitempty"`
	SubIssues      domain.SubIssueSummary `json:"sub_issues"`
	BlockedByCount int                    `json:"blocked_by_count"`
	BlockedValue   string                 `json:"blocked_value,omitempty"`
	Fields         []readItemField        `json:"fields"`
	Dates          readItemDates          `json:"dates"`
	BodyExcerpt    string                 `json:"body_excerpt"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	ClosedAt       string                 `json:"closed_at,omitempty"`
}

type readItemField struct {
	Field     string `json:"field"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type readItemDates struct {
	Available bool   `json:"available"`
	Source    string `json:"source,omitempty"`
	Start     string `json:"start,omitempty"`
	Target    string `json:"target,omitempty"`
}

type readItemMilestone struct {
	Title string `json:"title"`
	State string `json:"state"`
	DueOn string `json:"due_on,omitempty"`
}

type readItemParent struct {
	Ref   string `json:"ref"`
	Title string `json:"title"`
}

func readItemCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:         "item <ref>",
		Annotations: map[string]string{mcpArgsAnnotation: "ref: Issue reference: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number"},
		Short:       "One issue with its board fields, parent, sub-issue progress, blockers and dates",
		GroupID:     GroupRead,
		Args:        cobra.ExactArgs(1),
		RunE:        func(cmd *cobra.Command, args []string) error { return readItem(cmd.Context(), deps, args[0]) },
	}
}

func readItem(ctx context.Context, deps *Deps, raw string) error {
	ref, err := domain.ParseReference(raw)
	if err != nil {
		return err
	}
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	ref, err = domain.ResolveReference(ref, s.cfg.Repository)
	if err != nil {
		return err
	}
	it, err := deps.Reader.GetItem(ctx, s.project, ref)
	if err != nil {
		return readAPI(err)
	}
	payload := readItemPayloadOf(s, it)
	doc := readItemDocument(payload)
	doc.Data = payload
	return readRender(deps, doc)
}

func readItemPayloadOf(s *readSession, it domain.Item) readItemPayload {
	issue := it.Issue
	p := readItemPayload{
		GeneratedAt: readStamp(s.now), Ref: issue.Ref(), NodeID: issue.NodeID, ProjectItemID: it.ProjectItemID,
		Archived: it.Archived, Title: render.Title(issue.Title), State: string(issue.State), URL: issue.URL,
		Labels: readLabelList(issue.Labels), Assignees: readLoginList(issue.Assignees), SubIssues: issue.SubIssues,
		BlockedByCount: issue.BlockedByCount, Fields: readItemFields(s.project, it), Dates: readItemDatesOf(s.cfg, it),
		BodyExcerpt: render.Excerpt(render.LabelBody, issue.Body), CreatedAt: readStamp(issue.CreatedAt), UpdatedAt: readStamp(issue.UpdatedAt),
	}
	if issue.Type != nil {
		p.Type = issue.Type.Name
	}
	if issue.ClosedAt != nil {
		p.ClosedAt = readStamp(*issue.ClosedAt)
	}
	if m := issue.Milestone; m != nil {
		p.Milestone = &readItemMilestone{Title: render.Title(m.Title), State: m.State}
		if m.DueOn != nil {
			p.Milestone.DueOn = domain.DateText(*m.DueOn)
		}
	}
	if par := issue.Parent; par != nil {
		p.Parent = &readItemParent{Ref: fmt.Sprintf("%s/%s#%d", par.Owner, par.Repo, par.Number), Title: render.Title(par.Title)}
	}
	if blocked := domain.CapabilitiesOf(s.cfg).Blocked; blocked != nil {
		p.BlockedValue = it.Text(blocked.Field)
	}
	return p
}

// readItemFields lists the board values in project field order, then any
// value whose field the project no longer declares. Free text (the title
// and text fields) is external text and arrives delimited (section 6.8);
// option names, iteration titles, numbers and dates are board
// configuration and stay as they are.
func readItemFields(project domain.Project, it domain.Item) []readItemField {
	out := make([]readItemField, 0, len(it.Values))
	seen := map[string]bool{}
	add := func(v domain.FieldValue, free bool) {
		f := readItemField{Field: v.Field, Value: v.Value}
		if free {
			f.Value = render.Delimit(render.LabelValue, v.Value, render.TitleLimit)
		}
		if !v.UpdatedAt.IsZero() {
			f.UpdatedAt = readStamp(v.UpdatedAt)
		}
		out = append(out, f)
	}
	for _, field := range project.Fields {
		if v, ok := it.Values[field.Name]; ok {
			add(v, field.DataType == domain.DataTypeText || field.DataType == domain.DataTypeTitle)
			seen[field.Name] = true
		}
	}
	extra := make([]string, 0)
	for name := range it.Values {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		add(it.Values[name], true)
	}
	return out
}

func readItemDatesOf(cfg *domain.Config, it domain.Item) readItemDates {
	dates := domain.CapabilitiesOf(cfg).Dates
	if dates == nil {
		return readItemDates{}
	}
	return readItemDates{
		Available: true, Source: string(dates.Source),
		Start:  domain.ItemDateText(it, dates, domain.DateStart),
		Target: domain.ItemDateText(it, dates, domain.DateTarget),
	}
}

func readItemDocument(p readItemPayload) *render.Document {
	doc := render.NewDocument("Item " + p.Ref)
	sec := doc.AddSection("Issue")
	sec.AddKeyValue(readKeyTitle, p.Title).AddKeyValue("State", p.State).AddKeyValue("Type", p.Type).AddKeyValue("URL", p.URL)
	sec.AddKeyValue("Labels", readJoin(p.Labels)).AddKeyValue("Assignees", readJoin(p.Assignees))
	if p.Milestone != nil {
		sec.AddKeyValue("Milestone", fmt.Sprintf("%s (%s, due %s)", p.Milestone.Title, p.Milestone.State, p.Milestone.DueOn))
	}
	if p.Parent != nil {
		sec.AddKeyValue("Parent", p.Parent.Ref+" "+p.Parent.Title)
	}
	sec.AddKeyValue("Sub-issues", readProgress(p.SubIssues.Completed, p.SubIssues.Total))
	sec.AddKeyValue("Blocked by", fmt.Sprintf("%d open issue(s)", p.BlockedByCount))
	if p.BlockedValue != "" {
		sec.AddKeyValue("Blocked value", p.BlockedValue)
	}
	sec.AddKeyValue("Created", p.CreatedAt).AddKeyValue("Updated", p.UpdatedAt).AddKeyValue("Closed", p.ClosedAt)
	fields := doc.AddSection("Board fields")
	t := fields.SetTable("FIELD", "VALUE", "UPDATED")
	for _, f := range p.Fields {
		t.AddRow(f.Field, f.Value, f.UpdatedAt)
	}
	if p.Archived {
		fields.AddNote("archived item")
	}
	readItemDatesSection(doc.AddSection("Dates"), p.Dates)
	doc.AddSection("Body").AddNote(p.BodyExcerpt)
	doc.AddSection("").AddKeyValue(readKeyGenerated, p.GeneratedAt)
	return doc
}

func readItemDatesSection(sec *render.Section, d readItemDates) {
	if !d.Available {
		sec.AddNote("dates " + readUnavailable)
		return
	}
	sec.AddKeyValue("Source", d.Source).AddKeyValue("Start", d.Start).AddKeyValue("Target", d.Target)
}

func readLabelList(labels []domain.Label) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.Name)
	}
	return out
}

func readLoginList(users []domain.User) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.Login)
	}
	return out
}

func readJoin(list []string) string {
	s := ""
	for i, v := range list {
		if i > 0 {
			s += ", "
		}
		s += v
	}
	return s
}
