package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func readContextCommand(deps *Deps) *cobra.Command {
	var budget int
	var forLogin string
	cmd := &cobra.Command{
		Use:     "context",
		Short:   "Snapshot of the board for a prompt: rules, sprint, my items, attention, triage, epics, deliveries",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return readContext(cmd.Context(), deps, budget, forLogin)
		},
	}
	cmd.Flags().IntVar(&budget, "budget", domain.DefaultContextBudget, "approximate token budget of the snapshot")
	cmd.Flags().StringVar(&forLogin, "for", "", "login whose open items fill the my items section (default: the viewer)")
	return cmd
}

// readContextPayload is the --json form of context: the snapshot with
// the generated_at stamp in the UTC form every payload shares and the
// count of board items left out because they are not issues.
type readContextPayload struct {
	GeneratedAt string `json:"generated_at"`
	domain.Snapshot
	OmittedItems int `json:"omitted_items"`
}

// readContext builds the snapshot of section 7 through the domain and
// prints it; --json carries the snapshot itself. The budget is measured
// on the bytes of the chosen format, so what leaves the process fits it.
func readContext(ctx context.Context, deps *Deps, budget int, forLogin string) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	format, err := readFormat(deps)
	if err != nil {
		return err
	}
	viewer, err := deps.Reader.Viewer(ctx)
	if err != nil {
		return readAPI(err)
	}
	items, err := s.summaries(ctx)
	if err != nil {
		return err
	}
	who := forLogin
	if who == "" {
		who = viewer.Login
	}
	build := func(snap domain.Snapshot) *render.Document {
		return readContextDocument(readDelimitSnapshot(snap), s, who)
	}
	snap := domain.BuildSnapshot(domain.SnapshotInput{
		Config: s.cfg, Project: s.project, Items: items, Viewer: viewer.Login, For: forLogin, Now: s.now, Budget: budget,
		Measure: func(snap domain.Snapshot) int { return readMeasure(build(snap), format) },
	})
	return readRender(deps, build(snap))
}

// readMeasure renders the document in the format and converts its size
// to tokens with the domain rule of thumb.
func readMeasure(doc *render.Document, format render.Format) int {
	var buf bytes.Buffer
	if err := render.Render(&buf, doc, format); err != nil {
		return 0
	}
	return (buf.Len() + domain.TokenChars - 1) / domain.TokenChars
}

// readDelimitSnapshot wraps every external title in data delimiters so
// the snapshot reads as data in every format, JSON included.
func readDelimitSnapshot(s domain.Snapshot) domain.Snapshot {
	s.Title = render.Title(s.Title)
	s.Mine = readDelimitSummaries(s.Mine)
	s.Triage = readDelimitSummaries(s.Triage)
	s.Deliveries = readDelimitSummaries(s.Deliveries)
	s.Epics = readDelimitEpics(s.Epics)
	s.Attention = readDelimitAlerts(s.Attention)
	return s
}

func readContextDocument(snap domain.Snapshot, s *readSession, who string) *render.Document {
	doc := render.NewDocument("Context " + snap.Project.String())
	doc.Data = readContextPayload{Snapshot: snap, GeneratedAt: readStamp(snap.GeneratedAt), OmittedItems: s.skipped}
	cols := readColumnsOf(s.cfg)
	readContextRules(doc.AddSection("Team rules"), snap, s.generic())
	readSprintSection(doc.AddSection("Sprint"), snap.Sprint, s.cfg)
	readSummaryTable(doc.AddSection("Open items of "+who), snap.Mine, cols)
	readAlertTable(doc.AddSection("Attention"), snap.Attention)
	readSummaryTable(doc.AddSection("Triage queue"), snap.Triage, cols)
	readEpicTable(doc.AddSection("Epics"), snap.Epics)
	readSummaryTable(doc.AddSection("Deliveries this week"), snap.Deliveries, cols)
	readContextFooter(doc.AddSection("Snapshot"), snap, s.skipped)
	return doc
}

func readContextRules(sec *render.Section, snap domain.Snapshot, generic bool) {
	sec.AddKeyValue(readKeyProject, snap.Project.String()+" "+snap.Title).AddKeyValue("Viewer", snap.Viewer)
	if generic {
		sec.AddNote("generic mode: no board.yml, team rules " + readUnavailable)
		return
	}
	if snap.Rules.Language != "" {
		sec.AddKeyValue("Language", snap.Rules.Language)
	}
	groups := []struct {
		label string
		lines []string
	}{{"flow", snap.Rules.Flow}, {"wip", snap.Rules.WIP}, {"sla", snap.Rules.SLA}, {"ritual", snap.Rules.Rituals}}
	for _, g := range groups {
		for _, line := range g.lines {
			sec.AddItem(g.label + ": " + line)
		}
	}
}

// readContextFooter always closes the snapshot with generated_at,
// completeness, the omitted counts per section (F10) and the board items
// left out because they are not issues.
func readContextFooter(sec *render.Section, snap domain.Snapshot, skipped int) {
	sec.AddKeyValue(readKeyGenerated, readStamp(snap.GeneratedAt))
	sec.AddKeyValue("Completeness", string(snap.Completeness))
	var omitted []string
	for _, section := range domain.SnapshotSections {
		if n := snap.Omitted[section]; n > 0 {
			omitted = append(omitted, fmt.Sprintf("%s: %d", section, n))
		}
	}
	if len(omitted) == 0 {
		omitted = []string{readNoteNone}
	}
	sec.AddKeyValue("Omitted", strings.Join(omitted, ", "))
	if note := readSkippedNote(skipped); note != "" {
		sec.AddNote(note)
	}
}
