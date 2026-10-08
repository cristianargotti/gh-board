package commands

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readSummaryColumns lists the optional columns a board maps, so generic
// boards do not print empty lane, epic and sprint columns.
type readSummaryColumns struct {
	lane, epic, sprint, target bool
}

func readColumnsOf(cfg *domain.Config) readSummaryColumns {
	caps := domain.CapabilitiesOf(cfg)
	return readSummaryColumns{
		lane: caps.Lane != nil, epic: caps.Epic != nil, sprint: caps.Sprint != nil, target: caps.Dates != nil,
	}
}

// readSummaryTable writes item summaries whose titles the caller already
// delimited as external text (section 6.8).
func readSummaryTable(sec *render.Section, items []domain.ItemSummary, cols readSummaryColumns) {
	if len(items) == 0 {
		sec.AddNote(readNoteNone)
		return
	}
	header := []string{readColRef, readColStatus, readColAssignees, readColTitle}
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
	for _, it := range items {
		row := []string{it.Ref, it.Status, strings.Join(it.Assignees, ", "), it.Title}
		values := []string{it.Lane, it.Epic, it.Sprint, it.Target}
		for i, o := range optional {
			if o.on {
				row = append(row, values[i])
			}
		}
		t.AddRow(row...)
	}
}

// readAlertTable writes alerts whose item titles are already delimited;
// board-level alerts carry no item.
func readAlertTable(sec *render.Section, alerts []domain.Alert) {
	if len(alerts) == 0 {
		sec.AddNote(readNoteNone)
		return
	}
	t := sec.SetTable("RULE", "SEVERITY", "ITEM", readColTitle, "MESSAGE")
	for _, a := range alerts {
		item, title := readBoardLevel, ""
		if a.Item.NodeID != "" {
			item = fmt.Sprintf("%s#%d", a.Item.Repository, a.Item.Number)
			title = a.Item.Title
		}
		t.AddRow(a.RuleID, string(a.Severity), item, title, a.Message)
	}
}

// readEpicTable writes epic progress rows with delimited titles.
func readEpicTable(sec *render.Section, epics []domain.EpicProgress) {
	if len(epics) == 0 {
		sec.AddNote(readNoteNone)
		return
	}
	t := sec.SetTable(readColRef, readColStatus, "OWNER", "PROGRESS", readColStart, readColTarget, readColTitle)
	for _, e := range epics {
		t.AddRow(e.Ref, e.Status, e.Owner, readProgress(e.Completed, e.Total), e.Start, e.Target, e.Title)
	}
}

// readProgress renders sub-issue progress as "done/total (pct%)".
func readProgress(completed, total int) string {
	if total == 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d (%d%%)", completed, total, completed*100/total)
}

// readDelimitSummaries returns a copy with every title delimited, for the
// JSON payloads that carry summaries.
func readDelimitSummaries(in []domain.ItemSummary) []domain.ItemSummary {
	out := make([]domain.ItemSummary, len(in))
	for i, it := range in {
		it.Title = render.Title(it.Title)
		out[i] = it
	}
	return out
}

// readDelimitAlerts returns a copy with every item title delimited.
func readDelimitAlerts(in []domain.Alert) []domain.Alert {
	out := make([]domain.Alert, len(in))
	for i, a := range in {
		if a.Item.NodeID != "" {
			a.Item.Title = render.Title(a.Item.Title)
		}
		out[i] = a
	}
	return out
}

// readDelimitEpics returns a copy with every title delimited.
func readDelimitEpics(in []domain.EpicProgress) []domain.EpicProgress {
	out := make([]domain.EpicProgress, len(in))
	for i, e := range in {
		e.Title = render.Title(e.Title)
		out[i] = e
	}
	return out
}

// readSprintSection writes the sprint key values, or why there is none.
func readSprintSection(sec *render.Section, sprint *domain.SprintSummary, cfg *domain.Config) {
	if domain.CapabilitiesOf(cfg).Sprint == nil {
		sec.AddNote("sprint " + readUnavailable)
		return
	}
	if sprint == nil {
		sec.AddNote("no sprint in progress")
		return
	}
	sec.AddKeyValue(readKeyTitle, sprint.Title).AddKeyValue("Start", sprint.Start)
	sec.AddKeyValue("End", sprint.End).AddKeyValue("Days left", fmt.Sprint(sprint.DaysLeft))
	sec.AddKeyValue("Open", fmt.Sprint(sprint.Open)).AddKeyValue("Done", fmt.Sprint(sprint.Done))
}
