package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func readAttentionCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "attention",
		Short:   "Overdue, blocked, triage past SLA, WIP exceeded and the other alert rules",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readAttention(cmd.Context(), deps) },
	}
}

// readAttention evaluates the one alert function of section 9 and prints
// the result in the alerts.json shape, so --json matches what watch
// writes.
func readAttention(ctx context.Context, deps *Deps) error {
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
	state := domain.AlertState{
		GeneratedAt: s.now.UTC(), Project: s.project.Ref,
		Summary: domain.AlertSummary(sprint, alerts), Alerts: readDelimitAlerts(alerts),
	}
	doc := render.NewDocument("Attention " + state.Project.String())
	sec := doc.AddSection("")
	sec.AddKeyValue("Summary", state.Summary).AddKeyValue("Total", fmt.Sprint(len(state.Alerts)))
	if s.generic() {
		sec.AddNote("generic mode: rules that need a capability (dates, triage, sprint, WIP) find nothing")
	}
	readAlertTable(doc.AddSection("Alerts"), state.Alerts)
	doc.AddSection("").AddKeyValue(readKeyGenerated, readStamp(state.GeneratedAt))
	doc.Data = state
	return readRender(deps, doc)
}
