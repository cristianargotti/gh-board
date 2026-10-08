package commands

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

type planInitOptions struct {
	from, owner, repository, title, output string
	forms                                  bool
}

type planInitLocal struct {
	IssueTypes []domain.IssueType `json:"issue_types,omitempty"`
	Output     string             `json:"output"`
	Forms      bool               `json:"forms"`
	Config     *domain.Config     `json:"config"`
}

type planTemplateReader interface {
	TemplateConfig(context.Context, domain.ProjectRef) (*domain.Config, []domain.CreateLabelInput, []domain.CreateMilestoneInput, error)
}

type planOwnerResolver interface {
	OwnerID(context.Context, string) (string, error)
}

type planRepositoryResolver interface {
	RepositoryID(context.Context, string, string) (string, error)
}

func planInitCommand(deps *Deps) *cobra.Command {
	opts := &planInitOptions{}
	cmd := &cobra.Command{
		Use: planCommandInit, Short: "Plan a board setup and local configuration draft", GroupID: GroupPlan, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return planInit(cmd.Context(), deps, *opts) },
	}
	cmd.Flags().StringVar(&opts.from, "from", "", "template project as owner/number")
	cmd.Flags().StringVar(&opts.owner, "owner", "", "owner of the copied project, defaults to repository owner")
	cmd.Flags().StringVar(&opts.repository, "repository", "", "team repository as owner/name, overrides board.yml")
	cmd.Flags().StringVar(&opts.title, "title", "", "title of the copied project, defaults to the template title")
	cmd.Flags().StringVar(&opts.output, "output", config.FileName, "destination of the draft board.yml")
	cmd.Flags().BoolVar(&opts.forms, "forms", false, "render issue forms next to the draft under .github/ISSUE_TEMPLATE")
	return cmd
}

func planInit(ctx context.Context, deps *Deps, opts planInitOptions) error {
	cfg, err := planLoad(deps)
	if err != nil {
		return err
	}
	if opts.repository != "" {
		cfg.Repository = opts.repository
	}
	viewer, err := planViewer(ctx, deps)
	if err != nil {
		return err
	}
	pc := &planContext{deps: deps, cfg: cfg, viewer: viewer}
	if err := pc.planPermission(ctx, cfg.Repository); err != nil {
		return err
	}
	if opts.from != "" {
		return pc.planInitCopy(ctx, opts)
	}
	if cfg.Project.IsZero() {
		return domain.Errorf(domain.ExitUsage, "init needs --from or a configured project")
	}
	project, err := deps.Reader.DiscoverProject(ctx, cfg.Project)
	if err != nil {
		return planAPI(err)
	}
	if project.ViewerRole != domain.RoleAdmin && project.ViewerRole != domain.RoleWriter {
		return domain.Errorf(domain.ExitPolicy, "project write permission is required")
	}
	pc.project = project
	return pc.planInitSave(ctx, opts, cfg, nil)
}

func (pc *planContext) planInitSave(ctx context.Context, opts planInitOptions, template *domain.Config, steps []domain.Step) error {
	repositoryID, err := pc.planRepositoryID(ctx, pc.cfg.Repository)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(opts.output)
	if err != nil {
		return err
	}
	types, err := pc.planInitTypes(ctx, opts.owner)
	if err != nil {
		return err
	}
	local := planInitLocal{Output: output, Forms: opts.forms, Config: template, IssueTypes: types}
	if err := planInitPaths(local); err != nil {
		return err
	}
	payload, err := planJSON(local)
	if err != nil {
		return err
	}
	projectID := pc.project.NodeID
	if len(steps) > 0 && steps[0].Operation == domain.OpCopyProject {
		projectID = "step:0"
	}
	steps = append(steps, domain.Step{
		Operation: domain.OpLinkRepository,
		Target:    domain.Target{NodeID: projectID, Repository: pc.cfg.Repository, Title: pc.cfg.Repository},
		Field:     payload, After: repositoryID, Description: "Vincular o repositório " + pc.cfg.Repository + " e gerar o rascunho local de board.yml.",
	})
	return pc.planSave(planCommandInit, "Preparar o quadro da equipe e o rascunho de board.yml, sem alterar campos ou opções existentes.", steps)
}

func (pc *planContext) planRepositoryID(ctx context.Context, repository string) (string, error) {
	for _, repo := range pc.project.Repositories {
		if repo.FullName() == repository && repo.NodeID != "" {
			return repo.NodeID, nil
		}
	}
	owner, name, err := domain.SplitRepository(repository)
	if err != nil {
		return "", err
	}
	resolver, ok := pc.deps.Reader.(planRepositoryResolver)
	if !ok {
		return "", domain.Errorf(domain.ExitAPI, "repository node id discovery is required for init")
	}
	id, err := resolver.RepositoryID(ctx, owner, name)
	if err != nil {
		return "", planAPI(err)
	}
	if id == "" {
		return "", domain.Errorf(domain.ExitNotFound, "repository node id not found")
	}
	return id, nil
}

func (pc *planContext) planInitTypes(ctx context.Context, owner string) ([]domain.IssueType, error) {
	if owner == "" {
		owner = pc.project.Ref.Owner
	}
	if owner == "" {
		owner, _, _ = domain.SplitRepository(pc.cfg.Repository)
	}
	types, err := pc.deps.Reader.IssueTypes(ctx, owner)
	if err != nil {
		return nil, planAPI(err)
	}
	clean := append([]domain.IssueType(nil), types...)
	for i := range clean {
		clean[i].Name = render.Sanitize(clean[i].Name, render.TitleLimit)
	}
	return clean, nil
}
