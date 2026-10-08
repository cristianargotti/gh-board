package commands

import (
	"sort"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readRoadmapSprints marks the iterations that overlap the window.
func readRoadmapSprints(s *readSession, w readMonthWindow) []readRoadmapSprint {
	out := []readRoadmapSprint{}
	caps := domain.CapabilitiesOf(s.cfg)
	if caps.Sprint == nil {
		return out
	}
	field, ok := s.project.FieldByName(caps.Sprint.Field)
	if !ok {
		return out
	}
	iterations := append([]domain.Iteration(nil), field.Iterations...)
	sort.SliceStable(iterations, func(i, j int) bool { return iterations[i].Start.Before(iterations[j].Start) })
	for _, it := range iterations {
		months := w.months(it.Start, it.End())
		if len(months) == 0 {
			continue
		}
		out = append(out, readRoadmapSprint{
			Title: it.Title, Start: domain.DateText(it.Start), End: domain.DateText(it.LastDay()),
			State: readIterationState(it, domain.Today(s.now, w.loc)), Months: months,
		})
	}
	return out
}

// readRoadmapEpics spans every open epic over the months between its
// start and target dates; an epic with one date marks that month only.
func readRoadmapEpics(s *readSession, items []domain.Item, w readMonthWindow) []readRoadmapEpic {
	out := []readRoadmapEpic{}
	for _, e := range domain.EpicsProgress(s.cfg, items) {
		start, hasStart := domain.ParseDate(e.Start, w.loc)
		target, hasTarget := domain.ParseDate(e.Target, w.loc)
		e.Title = render.Title(e.Title)
		row := readRoadmapEpic{EpicProgress: e, Months: []string{}}
		switch {
		case hasStart && hasTarget:
			row.Months = w.months(start, target.AddDate(0, 0, 1))
		case hasStart:
			row.Months = w.months(start, start.AddDate(0, 0, 1))
		case hasTarget:
			row.Months = w.months(target, target.AddDate(0, 0, 1))
		}
		out = append(out, row)
	}
	return out
}

// readRoadmapMilestones merges the milestones the items carry with the
// repository milestones, counting open and done items per milestone.
func readRoadmapMilestones(s *readSession, items []domain.Item, repoMilestones []domain.Milestone, w readMonthWindow) []readRoadmapMilestone {
	rows := readMilestoneRows{byKey: map[string]*readRoadmapMilestone{}, window: w}
	for _, m := range repoMilestones {
		rows.add(s.cfg.Repository, m)
	}
	for _, it := range items {
		if it.Issue.Milestone == nil || it.Archived {
			continue
		}
		row := rows.add(it.Issue.Owner+"/"+it.Issue.Repo, *it.Issue.Milestone)
		if domain.IsDone(s.cfg, it) {
			row.Done++
		} else {
			row.Open++
		}
	}
	out := make([]readRoadmapMilestone, 0, len(rows.order))
	for _, key := range rows.order {
		out = append(out, *rows.byKey[key])
	}
	readSortMilestones(out)
	return out
}

// readMilestoneRows collects milestone rows by repository and title, in
// first-seen order.
type readMilestoneRows struct {
	byKey  map[string]*readRoadmapMilestone
	order  []string
	window readMonthWindow
}

// add returns the row of a milestone, creating it with its due month.
func (r *readMilestoneRows) add(repo string, m domain.Milestone) *readRoadmapMilestone {
	key := repo + "#" + m.Title
	if row, ok := r.byKey[key]; ok {
		return row
	}
	row := &readRoadmapMilestone{Repository: repo, Title: render.Title(m.Title), State: m.State, Months: []string{}}
	if m.DueOn != nil {
		due := domain.CalendarDate(*m.DueOn)
		row.DueOn = domain.DateText(due)
		row.Months = r.window.months(due, due.AddDate(0, 0, 1))
	}
	r.byKey[key], r.order = row, append(r.order, key)
	return row
}

// readSortMilestones orders by due date, undated last, then by title.
func readSortMilestones(rows []readRoadmapMilestone) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].DueOn, rows[j].DueOn
		if (a == "") != (b == "") {
			return a != ""
		}
		if a != b {
			return a < b
		}
		return rows[i].Title < rows[j].Title
	})
}
