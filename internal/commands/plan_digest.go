package commands

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func planDigestCommand(deps *Deps) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use: "post", Short: "Plan publishing the weekly digest", GroupID: GroupPlan, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !domain.Contains([]string{"on_track", "at_risk", "off_track"}, status) {
				return domain.Errorf(domain.ExitUsage, "--status must be on_track, at_risk or off_track")
			}
			pc, err := planStart(cmd.Context(), deps)
			if err != nil {
				return err
			}
			return pc.planDigest(cmd.Context(), status)
		},
	}
	cmd.Flags().StringVar(&status, "status", "on_track", "project status: on_track, at_risk or off_track")
	return cmd
}

func (pc *planContext) planDigest(ctx context.Context, status string) error {
	lister, ok := pc.deps.Reader.(plan.StatusUpdateLister)
	if !ok {
		return domain.Errorf(domain.ExitAPI, "status update history is required for weekly idempotency")
	}
	now := pc.deps.Now().In(domain.LocationOf(pc.cfg))
	updates, err := lister.ListStatusUpdates(ctx, pc.project.NodeID)
	if err != nil {
		return planAPI(err)
	}
	marker := plan.WeekMarker(now)
	if planDigestExists(updates, marker) {
		return planMessage(pc.deps, "This ISO week already has a digest.")
	}
	items, err := pc.planItems(ctx)
	if err != nil {
		return err
	}
	input, err := pc.planDigestInput(ctx, items)
	if err != nil {
		return err
	}
	digest, err := domain.BuildDigest(input)
	if err != nil {
		return err
	}
	payload, err := planJSON(domain.StatusUpdateInput{ProjectID: pc.project.NodeID, Body: digest.Text + "\n\n" + marker, Status: strings.ToUpper(status), StartDate: &digest.Start, TargetDate: &digest.End})
	if err != nil {
		return err
	}
	description := "Publicar o resumo semanal " + digest.Week + "."
	step := domain.Step{Operation: domain.OpCreateStatusUpdate, Target: domain.Target{NodeID: pc.project.NodeID, Title: pc.project.Title}, Field: digest.Week, After: payload, Description: description}
	return pc.planSave("digest post", description, []domain.Step{step})
}

func planDigestExists(updates []plan.StatusUpdate, marker string) bool {
	for _, update := range updates {
		if strings.Contains(update.Body, marker) || (!update.CreatedAt.IsZero() && plan.WeekMarker(update.CreatedAt) == marker) {
			return true
		}
	}
	return false
}

func (pc *planContext) planDigestInput(ctx context.Context, items []domain.Item) (domain.DigestInput, error) {
	input := domain.DigestInput{Config: pc.cfg, Project: pc.project, Items: items, Now: pc.deps.Now(), Timelines: make(map[string]domain.IssueTimeline)}
	if pc.cfg.Repository != "" {
		milestones, err := pc.planMilestones(ctx, pc.cfg.Repository)
		if err != nil {
			return input, err
		}
		for _, milestone := range milestones {
			input.Milestones = append(input.Milestones, domain.DigestMilestone{Repository: pc.cfg.Repository, Milestone: milestone})
		}
	}
	reader, ok := pc.deps.Reader.(domain.IssueTimelineReader)
	if !ok || !domain.Contains(pc.cfg.Digest.Metrics, "first_response") {
		return input, nil
	}
	for _, item := range items {
		if pc.cfg.Capabilities.Triage == nil || !item.HasLabel(pc.cfg.Capabilities.Triage.Label) {
			continue
		}
		timeline, err := reader.IssueTimeline(ctx, item.Issue.NodeID)
		if err != nil {
			return input, planAPI(err)
		}
		input.Timelines[item.Issue.NodeID] = timeline
	}
	return input, nil
}
