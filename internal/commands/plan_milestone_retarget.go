package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func planMilestoneRetargetCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use: "retarget <from-title> <to-title>", Short: "Plan moving board issues between repository milestones", GroupID: GroupPlan, Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pc, err := planStart(cmd.Context(), deps)
			if err != nil {
				return err
			}
			return pc.planRetarget(cmd.Context(), args[0], args[1])
		},
	}
}

func (pc *planContext) planRetarget(ctx context.Context, from, to string) error {
	if err := pc.planPermission(ctx, pc.cfg.Repository); err != nil {
		return err
	}
	milestones, err := pc.planMilestones(ctx, pc.cfg.Repository)
	if err != nil {
		return err
	}
	source, err := planResolveMilestone(milestones, from)
	if err != nil {
		return err
	}
	target, err := planResolveMilestone(milestones, to)
	if err != nil {
		return err
	}
	if source.ID == target.ID {
		return planMessage(pc.deps, "Milestones are identical.")
	}
	items, err := pc.planItems(ctx)
	if err != nil {
		return err
	}
	selected, steps := planRetargetSteps(items, pc.cfg.Repository, source, target)
	if err := pc.planCheckItems(ctx, selected); err != nil {
		return err
	}
	description := fmt.Sprintf("Transferir as issues do marco #%d para o marco #%d em %s.", source.Number, target.Number, pc.cfg.Repository)
	return pc.planSave("milestone retarget", description, steps)
}

func planRetargetSteps(items []domain.Item, repository string, from, to domain.Milestone) ([]domain.Item, []domain.Step) {
	var selected []domain.Item
	var steps []domain.Step
	for _, item := range items {
		if item.Archived || item.Issue.Milestone == nil || item.Issue.Milestone.ID != from.ID || item.Issue.Owner+"/"+item.Issue.Repo != repository {
			continue
		}
		selected = append(selected, item)
		steps = append(steps, planMilestoneStep(item, to))
	}
	return selected, steps
}
