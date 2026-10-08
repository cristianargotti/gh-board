package commands

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readStatusPayload is the --json form of status.
type readStatusPayload struct {
	GeneratedAt string                `json:"generated_at"`
	Project     readProjectSummary    `json:"project"`
	Generic     bool                  `json:"generic"`
	StatusField string                `json:"status_field"`
	StatusFound bool                  `json:"status_field_found"`
	Items       readItemCounts        `json:"items"`
	Statuses    []readStatusCount     `json:"statuses"`
	Sprint      *domain.SprintSummary `json:"sprint"`
	Alerts      map[string]int        `json:"alerts"`
	AlertCount  int                   `json:"alert_count"`
	Summary     string                `json:"summary"`
}

type readProjectSummary struct {
	Ref          string   `json:"ref"`
	Title        string   `json:"title"`
	ItemCount    int      `json:"item_count"`
	ViewerRole   string   `json:"viewer_role"`
	Repositories []string `json:"repositories"`
}

type readItemCounts struct {
	Open     int `json:"open"`
	Closed   int `json:"closed"`
	Archived int `json:"archived"`
	// Omitted counts the board items that are not issues.
	Omitted int `json:"omitted"`
}

type readStatusCount struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Open  int    `json:"open"`
}

func readStatusCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Board overview: items per status, sprint and alert counts",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readStatus(cmd.Context(), deps) },
	}
}

func readStatus(ctx context.Context, deps *Deps) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	items, err := s.summaries(ctx)
	if err != nil {
		return err
	}
	alerts := domain.EvaluateAlerts(domain.AlertInput{Config: s.cfg, Project: s.project, Items: items, Now: s.now})
	sprint := domain.SprintSummaryOf(s.cfg, s.project, items, s.now)
	_, found := domain.StatusField(s.cfg, s.project)
	payload := readStatusPayload{
		GeneratedAt: readStamp(s.now), Project: readProjectSummaryOf(s.project), Generic: s.generic(),
		StatusField: domain.StatusFieldName(s.cfg), StatusFound: found,
		Items: readCountItems(items, s.skipped), Statuses: readStatusCounts(s.cfg, s.project, items), Sprint: sprint,
		Alerts: domain.AlertState{Alerts: alerts}.CountByRule(), AlertCount: len(alerts),
		Summary: domain.AlertSummary(sprint, alerts),
	}
	doc := readStatusDocument(payload, s.cfg)
	doc.Data = payload
	return readRender(deps, doc)
}

func readProjectSummaryOf(p domain.Project) readProjectSummary {
	repos := make([]string, 0, len(p.Repositories))
	for _, r := range p.Repositories {
		repos = append(repos, r.FullName())
	}
	return readProjectSummary{
		Ref: p.Ref.String(), Title: render.Title(p.Title), ItemCount: p.ItemCount,
		ViewerRole: string(p.ViewerRole), Repositories: repos,
	}
}

func readCountItems(items []domain.Item, skipped int) readItemCounts {
	c := readItemCounts{Omitted: skipped}
	for _, it := range items {
		switch {
		case it.Archived:
			c.Archived++
		case it.IsOpen():
			c.Open++
		default:
			c.Closed++
		}
	}
	return c
}

// readStatusCounts lists the open items per status option in board
// order, then any value the field no longer offers.
func readStatusCounts(cfg *domain.Config, project domain.Project, items []domain.Item) []readStatusCount {
	counts := domain.CountByStatus(items, domain.StatusFieldName(cfg))
	out := make([]readStatusCount, 0, len(counts))
	seen := map[string]bool{}
	if field, ok := domain.StatusField(cfg, project); ok {
		for _, name := range field.OptionNames() {
			out = append(out, readStatusCount{Name: name, Class: string(domain.ClassifyStatus(cfg, name)), Open: counts[name]})
			seen[name] = true
		}
	}
	extra := make([]string, 0)
	for name := range counts {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		out = append(out, readStatusCount{Name: name, Class: string(domain.ClassifyStatus(cfg, name)), Open: counts[name]})
	}
	return out
}

func readStatusDocument(p readStatusPayload, cfg *domain.Config) *render.Document {
	doc := render.NewDocument("Board " + p.Project.Ref)
	sec := doc.AddSection(readKeyProject)
	sec.AddKeyValue(readKeyTitle, p.Project.Title).AddKeyValue(readKeyRole, p.Project.ViewerRole)
	sec.AddKeyValue(readKeyItems, fmt.Sprintf("%d open, %d closed, %d archived", p.Items.Open, p.Items.Closed, p.Items.Archived))
	if note := readSkippedNote(p.Items.Omitted); note != "" {
		sec.AddNote(note)
	}
	if p.Generic {
		sec.AddNote("generic mode: no board.yml, fields are shown by their real names")
	}
	statuses := doc.AddSection("Open items per status")
	if !p.StatusFound {
		statuses.AddNote("no single select field named " + p.StatusField + " on the board")
	}
	t := statuses.SetTable(readColStatus, "CLASS", "OPEN")
	for _, st := range p.Statuses {
		t.AddRow(st.Name, st.Class, fmt.Sprint(st.Open))
	}
	readSprintSection(doc.AddSection("Sprint"), p.Sprint, cfg)
	alerts := doc.AddSection("Alerts")
	alerts.AddKeyValue("Summary", p.Summary).AddKeyValue("Total", fmt.Sprint(p.AlertCount))
	for _, rule := range domain.AlertRuleIDs {
		if n := p.Alerts[rule]; n > 0 {
			alerts.AddItem(fmt.Sprintf("%s: %d", rule, n))
		}
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, p.GeneratedAt)
	return doc
}
