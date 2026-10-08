package commands

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func (pc *planContext) planBuild(command, description string, steps []domain.Step) (domain.Plan, error) {
	now := pc.deps.Now()
	id, err := plan.NewID(now)
	if err != nil {
		return domain.Plan{}, err
	}
	p := domain.Plan{
		ID: id, KitVersion: pc.deps.Version, CreatedAt: now, ExpiresAt: now.Add(domain.PlanExpiry),
		Actor: pc.viewer.Login, Host: pc.viewer.Host, Project: pc.project.Ref, ProjectID: pc.project.NodeID,
		Command: command, Description: description, Steps: steps,
	}
	return p.Seal()
}

func (pc *planContext) planSave(command, description string, steps []domain.Step) error {
	if len(steps) == 0 {
		return planMessage(pc.deps, "No changes required.")
	}
	if _, err := planFormat(pc.deps); err != nil {
		return err
	}
	p, err := pc.planBuild(command, description, steps)
	if err != nil {
		return err
	}
	var path string
	if !pc.deps.Flags.DryRun {
		if pc.deps.Dirs.State == "" {
			return domain.Errorf(domain.ExitUsage, "a state directory is required")
		}
		path, err = plan.Write(pc.deps.Dirs.State, p)
		if err != nil {
			return err
		}
	}
	if err := pc.planAudit(p, "planned", nil); err != nil {
		return err
	}
	if err := planPresent(pc.deps, p, path); err != nil {
		return err
	}
	if pc.deps.Flags.DryRun {
		return domain.Errorf(domain.ExitPlanRequired, "dry run: no plan saved; rerun without --dry-run to create an applicable plan")
	}
	return domain.Errorf(domain.ExitPlanRequired, "plan requires human apply: %s", p.ID)
}

func (pc *planContext) planAudit(p domain.Plan, result string, failure error) error {
	entry := audit.Entry{
		At: pc.deps.Now(), Actor: pc.viewer.Login, Host: pc.viewer.Host,
		Project: p.Project, Command: p.Command, Reason: pc.deps.Flags.Reason, Targets: p.Targets(), Items: p.Items(),
		Result: result, DryRun: pc.deps.Flags.DryRun, PlanID: p.ID,
	}
	for _, step := range p.Steps {
		field := step.Field
		if step.Operation == domain.OpLinkRepository {
			// The field of the link step carries the local init payload, not
			// a name worth an audit line.
			field = ""
		}
		entry.Changes = append(entry.Changes, audit.Change{Target: step.Target.NodeID, Field: field, Before: step.Before, After: step.After})
	}
	if failure != nil {
		entry.Error = failure.Error()
	}
	return errors.Join(failure, audit.Append(pc.deps.Dirs.State, entry))
}

func planJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func (pc *planContext) planCheckItems(ctx context.Context, items []domain.Item) error {
	seen := make(map[string]bool)
	for _, item := range items {
		repo := item.Issue.Owner + "/" + item.Issue.Repo
		if !seen[repo] {
			if err := pc.planPermission(ctx, repo); err != nil {
				return err
			}
			seen[repo] = true
		}
		if err := planExpect(item, pc.deps.Flags.Expect); err != nil {
			return err
		}
	}
	return nil
}
