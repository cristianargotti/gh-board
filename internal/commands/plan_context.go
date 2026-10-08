package commands

import (
	"context"
	"fmt"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

type planContext struct {
	deps    *Deps
	cfg     *domain.Config
	project domain.Project
	viewer  domain.Viewer
}

func planLoad(deps *Deps) (*domain.Config, error) {
	var ref domain.ProjectRef
	var err error
	if deps.Flags.Project != "" {
		ref, err = domain.ParseProjectRef(deps.Flags.Project)
		if err != nil {
			return nil, err
		}
	}
	if deps.Config == nil {
		loaded, loadErr := config.Load(config.LoadOptions{Path: deps.Flags.Config, Dirs: deps.Dirs, Project: ref})
		if loadErr != nil {
			return nil, loadErr
		}
		deps.Config = &loaded
	}
	cfg := domain.Config{Version: config.SupportedVersion}
	if deps.Config.Config != nil {
		cfg = *deps.Config.Config
	}
	if !ref.IsZero() {
		cfg.Project = ref
	}
	return &cfg, nil
}

func planStart(ctx context.Context, deps *Deps) (*planContext, error) {
	cfg, err := planLoad(deps)
	if err != nil {
		return nil, err
	}
	if cfg.Project.IsZero() {
		return nil, readNoProject(deps.Config)
	}
	viewer, err := planViewer(ctx, deps)
	if err != nil {
		return nil, err
	}
	project, err := deps.Reader.DiscoverProject(ctx, cfg.Project)
	if err != nil {
		return nil, planAPI(err)
	}
	if project.ViewerRole != domain.RoleAdmin && project.ViewerRole != domain.RoleWriter {
		return nil, domain.Errorf(domain.ExitPolicy, "write permission is required on project %s", cfg.Project)
	}
	return &planContext{deps: deps, cfg: cfg, project: project, viewer: viewer}, nil
}

func planViewer(ctx context.Context, deps *Deps) (domain.Viewer, error) {
	if deps.Reader == nil || deps.Clock == nil || deps.Dirs.State == "" {
		return domain.Viewer{}, domain.Errorf(domain.ExitUsage, "a reader, clock and state directory are required")
	}
	viewer, err := deps.Reader.Viewer(ctx)
	if err != nil {
		return viewer, planAPI(err)
	}
	if viewer.Login == "" || viewer.Host == "" {
		return viewer, domain.Errorf(domain.ExitAPI, "viewer identity is incomplete")
	}
	if viewer.Shadowed() && deps.Err != nil {
		_, err = fmt.Fprintf(deps.Err, "Warning: %s shadows the gh keyring token.\n", render.Sanitize(viewer.TokenSource, 0))
	}
	return viewer, err
}

func planAPI(err error) error {
	if err == nil {
		return nil
	}
	if domain.CodeOf(err) != domain.ExitUsage {
		return err
	}
	return domain.NewError(domain.ExitAPI, err)
}

func (pc *planContext) planItems(ctx context.Context) ([]domain.Item, error) {
	var items []domain.Item
	var cursor string
	seen := make(map[string]bool)
	for {
		page, err := pc.deps.Reader.ListItems(ctx, pc.project, domain.ListOptions{All: true, Cursor: cursor})
		if err != nil {
			return nil, planAPI(err)
		}
		items = append(items, page.Items...)
		if !page.HasNext {
			return items, nil
		}
		if page.NextCursor == "" || seen[page.NextCursor] {
			return nil, domain.Errorf(domain.ExitAPI, "item pagination did not advance")
		}
		cursor, seen[page.NextCursor] = page.NextCursor, true
	}
}
