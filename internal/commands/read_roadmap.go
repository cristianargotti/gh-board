package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// Roadmap bounds and the glyphs of the month timeline.
const (
	readRoadmapDefaultMonths = 3
	readRoadmapMaxMonths     = 24
	readRoadmapEpicGlyph     = "===="
	readRoadmapSprintGlyph   = "~~~~"
	readRoadmapDueGlyph      = "<>"
)

// readMonthWindow is the timeline: N months from the first day of the
// current month in the board location, as calendar dates (dates.go).
type readMonthWindow struct {
	from, to time.Time
	loc      *time.Location
	labels   []string
}

// readRoadmapPayload is the --json form of roadmap.
type readRoadmapPayload struct {
	GeneratedAt string                 `json:"generated_at"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Months      []string               `json:"months"`
	Sprints     []readRoadmapSprint    `json:"sprints"`
	Epics       []readRoadmapEpic      `json:"epics"`
	Milestones  []readRoadmapMilestone `json:"milestones"`
	Notes       []string               `json:"notes,omitempty"`
}

type readRoadmapSprint struct {
	Title  string   `json:"title"`
	Start  string   `json:"start"`
	End    string   `json:"end"`
	State  string   `json:"state"`
	Months []string `json:"months"`
}

type readRoadmapEpic struct {
	domain.EpicProgress
	Months []string `json:"months"`
}

type readRoadmapMilestone struct {
	Repository string   `json:"repository"`
	Title      string   `json:"title"`
	State      string   `json:"state"`
	DueOn      string   `json:"due_on,omitempty"`
	Open       int      `json:"open"`
	Done       int      `json:"done"`
	Months     []string `json:"months"`
}

func readRoadmapCommand(deps *Deps) *cobra.Command {
	var months int
	cmd := &cobra.Command{
		Use:     "roadmap",
		Short:   "Epics and milestones on a month timeline with the sprint iterations as markers",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readRoadmap(cmd.Context(), deps, months) },
	}
	cmd.Flags().IntVar(&months, "months", readRoadmapDefaultMonths, "months on the timeline, starting with the current one")
	return cmd
}

// readRoadmap reads the same dates the GitHub roadmap view reads: epic
// start and target dates, milestone due dates, iteration start and
// duration.
func readRoadmap(ctx context.Context, deps *Deps, months int) error {
	if months < 1 || months > readRoadmapMaxMonths {
		return domain.Errorf(domain.ExitUsage, "--months must be between 1 and %d", readRoadmapMaxMonths)
	}
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	items, err := s.items(ctx)
	if err != nil {
		return err
	}
	repoMilestones, err := readRepositoryMilestones(ctx, s)
	if err != nil {
		return err
	}
	window := readWindowOf(s.now, domain.LocationOf(s.cfg), months)
	payload := readRoadmapPayload{
		GeneratedAt: readStamp(s.now), From: domain.DateText(window.from),
		To: domain.DateText(window.to.AddDate(0, 0, -1)), Months: window.labels,
		Sprints: readRoadmapSprints(s, window), Epics: readRoadmapEpics(s, items, window),
		Milestones: readRoadmapMilestones(s, items, repoMilestones, window),
	}
	payload.Notes = readRoadmapNotes(s, payload)
	doc := readRoadmapDocument(payload)
	doc.Data = payload
	return readRender(deps, doc)
}

// readRepositoryMilestones lists the milestones of the configured
// repository, the ones the roadmap shows even without items.
func readRepositoryMilestones(ctx context.Context, s *readSession) ([]domain.Milestone, error) {
	if s.cfg.Repository == "" {
		return nil, nil
	}
	owner, name, err := domain.SplitRepository(s.cfg.Repository)
	if err != nil {
		return nil, err
	}
	milestones, err := s.deps.Reader.RepositoryMilestones(ctx, owner, name)
	if err != nil {
		return nil, readAPI(err)
	}
	return milestones, nil
}

func readWindowOf(now time.Time, loc *time.Location, months int) readMonthWindow {
	today := domain.Today(now, loc)
	from := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	w := readMonthWindow{from: from, to: from.AddDate(0, months, 0), loc: loc}
	for i := range months {
		w.labels = append(w.labels, from.AddDate(0, i, 0).Format("2006-01"))
	}
	return w
}

// months lists the labels of the months a [start, end) span touches.
func (w readMonthWindow) months(start, end time.Time) []string {
	out := []string{}
	for i, label := range w.labels {
		monthStart := w.from.AddDate(0, i, 0)
		monthEnd := w.from.AddDate(0, i+1, 0)
		if start.Before(monthEnd) && end.After(monthStart) {
			out = append(out, label)
		}
	}
	return out
}

func readRoadmapNotes(s *readSession, p readRoadmapPayload) []string {
	var notes []string
	caps := domain.CapabilitiesOf(s.cfg)
	if caps.Epic == nil || caps.Epic.IssueType == "" {
		notes = append(notes, "epics "+readUnavailable)
	}
	if caps.Dates == nil {
		notes = append(notes, "dates "+readUnavailable+": epics carry no span")
	}
	if caps.Sprint == nil {
		notes = append(notes, "sprint "+readUnavailable+": no iteration markers")
	}
	undated := 0
	for _, e := range p.Epics {
		if e.Start == "" && e.Target == "" {
			undated++
		}
	}
	if undated > 0 {
		notes = append(notes, fmt.Sprintf("%d epic(s) without dates", undated))
	}
	return notes
}

func readRoadmapDocument(p readRoadmapPayload) *render.Document {
	doc := render.NewDocument("Roadmap")
	sec := doc.AddSection("")
	sec.AddKeyValue("From", p.From).AddKeyValue("To", p.To)
	columns := append([]string{readColKind, readColName, readColStart, readColTarget, "PROGRESS"}, p.Months...)
	t := sec.SetTable(append(columns, readColTitle)...)
	for _, sp := range p.Sprints {
		t.AddRow(readRoadmapRow("sprint", sp.Title, sp.Start, sp.End, sp.State, readRoadmapSprintGlyph, sp.Months, p.Months, "")...)
	}
	for _, e := range p.Epics {
		progress := readProgress(e.Completed, e.Total)
		t.AddRow(readRoadmapRow("epic", e.Ref, e.Start, e.Target, progress, readRoadmapEpicGlyph, e.Months, p.Months, e.Title)...)
	}
	for _, m := range p.Milestones {
		progress := readProgress(m.Done, m.Open+m.Done)
		t.AddRow(readRoadmapRow("milestone", m.Repository, "", m.DueOn, progress, readRoadmapDueGlyph, m.Months, p.Months, m.Title)...)
	}
	for _, note := range p.Notes {
		sec.AddNote(note)
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, p.GeneratedAt)
	return doc
}

// readRoadmapRow lays one row out: the fixed cells, one cell per month
// carrying the glyph where the row spans it, then the title.
func readRoadmapRow(kind, name, start, end, progress, glyph string, marked, months []string, title string) []string {
	row := []string{kind, name, start, end, progress}
	for _, month := range months {
		cell := ""
		if domain.Contains(marked, month) {
			cell = glyph
		}
		row = append(row, cell)
	}
	return append(row, title)
}
