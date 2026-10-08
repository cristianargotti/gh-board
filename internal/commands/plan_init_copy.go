package commands

import (
	"context"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func (pc *planContext) planInitCopy(ctx context.Context, opts planInitOptions) error {
	ref, err := domain.ParseProjectRef(opts.from)
	if err != nil {
		return err
	}
	source, err := pc.deps.Reader.DiscoverProject(ctx, ref)
	if err != nil {
		return planAPI(err)
	}
	cfg, labels, milestones, err := pc.planTemplate(ctx, ref)
	if err != nil {
		return planAPI(err)
	}
	if cfg == nil {
		// No board.yml describes the template: the team's own configuration
		// drives the draft and the forms, and no label or milestone is invented.
		cfg = pc.cfg
	}
	step, err := pc.planCopyStep(ctx, opts, source)
	if err != nil {
		return err
	}
	steps, err := pc.planTemplateSteps(ctx, labels, milestones)
	if err != nil {
		return err
	}
	local := *cfg
	local.Repository = pc.cfg.Repository
	return pc.planInitSave(ctx, opts, &local, append([]domain.Step{step}, steps...))
}

func (pc *planContext) planCopyStep(ctx context.Context, opts planInitOptions, source domain.Project) (domain.Step, error) {
	owner := opts.owner
	if owner == "" {
		owner, _, _ = domain.SplitRepository(pc.cfg.Repository)
	}
	ownerID := pc.viewer.ID
	if !strings.EqualFold(owner, pc.viewer.Login) {
		resolver, ok := pc.deps.Reader.(planOwnerResolver)
		if !ok {
			return domain.Step{}, domain.Errorf(domain.ExitAPI, "owner node id discovery is required for init")
		}
		var err error
		ownerID, err = resolver.OwnerID(ctx, owner)
		if err != nil {
			return domain.Step{}, planAPI(err)
		}
	}
	if ownerID == "" || source.NodeID == "" {
		return domain.Step{}, domain.Errorf(domain.ExitNotFound, "owner or template node id is missing")
	}
	title := opts.title
	if title == "" {
		title = source.Title
	}
	payload, err := planJSON(domain.CopyProjectInput{SourceProjectID: source.NodeID, OwnerID: ownerID, Title: title, IncludeDraftIssues: false})
	return domain.Step{
		Operation: domain.OpCopyProject, Target: domain.Target{NodeID: source.NodeID, Repository: pc.cfg.Repository, Title: title},
		After: payload, Description: "Copiar o projeto modelo " + source.Ref.String() + " para " + owner + ", preservando campos, opções e visualizações.",
	}, err
}

func (pc *planContext) planTemplateSteps(ctx context.Context, labels []domain.CreateLabelInput, milestones []domain.CreateMilestoneInput) ([]domain.Step, error) {
	id, err := pc.planRepositoryID(ctx, pc.cfg.Repository)
	if err != nil {
		return nil, err
	}
	owner, repo, err := domain.SplitRepository(pc.cfg.Repository)
	if err != nil {
		return nil, err
	}
	existingLabels, err := pc.deps.Reader.RepositoryLabels(ctx, owner, repo)
	if err != nil {
		return nil, planAPI(err)
	}
	existingMilestones, err := pc.planMilestones(ctx, pc.cfg.Repository)
	if err != nil {
		return nil, err
	}
	steps, err := planTemplateLabels(pc.cfg.Repository, id, labels, existingLabels)
	if err != nil {
		return nil, err
	}
	more, err := planTemplateMilestones(owner, repo, milestones, existingMilestones)
	return append(steps, more...), err
}

func planTemplateLabels(repository, repositoryID string, labels []domain.CreateLabelInput, existing []domain.Label) ([]domain.Step, error) {
	var steps []domain.Step
	for _, label := range labels {
		if _, err := domain.ResolveLabel(existing, label.Name); err == nil {
			continue
		}
		if strings.TrimSpace(label.Name) == "" {
			return nil, domain.Errorf(domain.ExitUsage, "template label name is empty")
		}
		label.RepositoryID = repositoryID
		payload, err := planJSON(label)
		if err != nil {
			return nil, err
		}
		steps = append(steps, domain.Step{
			Operation: domain.OpCreateLabel, Target: domain.Target{Repository: repository, Title: label.Name},
			After: payload, Description: "Criar a etiqueta " + domain.CleanTitle(label.Name) + ".",
		})
		existing = append(existing, domain.Label{Name: label.Name})
	}
	return steps, nil
}

func planTemplateMilestones(owner, repo string, milestones []domain.CreateMilestoneInput, existing []domain.Milestone) ([]domain.Step, error) {
	var steps []domain.Step
	for _, milestone := range milestones {
		if _, err := domain.ResolveMilestone(existing, milestone.Title); err == nil {
			continue
		}
		if strings.TrimSpace(milestone.Title) == "" {
			return nil, domain.Errorf(domain.ExitUsage, "template milestone title is empty")
		}
		milestone.Owner, milestone.Repo = owner, repo
		payload, err := planJSON(milestone)
		if err != nil {
			return nil, err
		}
		steps = append(steps, domain.Step{
			Operation: domain.OpCreateMilestone, Target: domain.Target{Repository: owner + "/" + repo, Title: milestone.Title},
			After: payload, Description: "Criar o marco " + domain.CleanTitle(milestone.Title) + ".",
		})
		existing = append(existing, domain.Milestone{Title: milestone.Title})
	}
	return steps, nil
}

// planTemplate returns the board.yml of the template project with its label
// and milestone declarations: from a reader that knows templates, else from
// the reference board.yml the kit embeds when it describes that project.
func (pc *planContext) planTemplate(ctx context.Context, ref domain.ProjectRef) (*domain.Config, []domain.CreateLabelInput, []domain.CreateMilestoneInput, error) {
	if reader, ok := pc.deps.Reader.(planTemplateReader); ok {
		return reader.TemplateConfig(ctx, ref)
	}
	return planEmbeddedTemplate(ref, domain.LocationOf(pc.cfg))
}

// planEmbeddedTemplate answers nil when the embedded reference board.yml
// describes another project, so init never applies one team's labels to a
// template that is not theirs.
func planEmbeddedTemplate(ref domain.ProjectRef, loc *time.Location) (*domain.Config, []domain.CreateLabelInput, []domain.CreateMilestoneInput, error) {
	reference, err := planReferenceConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	if !strings.EqualFold(reference.Project.Owner, ref.Owner) || reference.Project.Number != ref.Number {
		return nil, nil, nil, nil
	}
	milestones, err := reference.Template.MilestoneInputs("", "", loc)
	if err != nil {
		return nil, nil, nil, err
	}
	return reference, reference.Template.LabelInputs(""), milestones, nil
}
