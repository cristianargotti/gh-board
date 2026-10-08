package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func (pc *planContext) planMilestones(ctx context.Context, repository string) ([]domain.Milestone, error) {
	owner, repo, err := domain.SplitRepository(repository)
	if err != nil {
		return nil, err
	}
	milestones, err := pc.deps.Reader.RepositoryMilestones(ctx, owner, repo)
	return milestones, planAPI(err)
}

func planResolveMilestone(milestones []domain.Milestone, title string) (domain.Milestone, error) {
	var match *domain.Milestone
	for _, m := range milestones {
		if !strings.EqualFold(m.Title, title) {
			continue
		}
		if match != nil {
			return domain.Milestone{}, domain.Errorf(domain.ExitNotFound, "ambiguous milestone %s", render.Title(title))
		}
		snapshot := m
		match = &snapshot
	}
	if match != nil && match.ID != "" {
		return *match, nil
	}
	_, err := domain.ResolveMilestone(milestones, title)
	if err == nil {
		return domain.Milestone{}, domain.Errorf(domain.ExitAPI, "milestone has no node id")
	}
	return domain.Milestone{}, domain.NewError(domain.CodeOf(err), errors.New(render.Sanitize(err.Error(), 0)))
}

func planMilestoneStep(item domain.Item, milestone domain.Milestone) domain.Step {
	step := domain.Step{
		Operation: domain.OpUpdateIssue, Field: domain.UpdateFieldMilestone, Target: planTarget(item), After: milestone.Title,
		Description: fmt.Sprintf("Definir o marco #%d na issue %s.", milestone.Number, item.Issue.Ref()),
	}
	step.Before, _ = domain.StepCurrentValue(step, item)
	return step
}
