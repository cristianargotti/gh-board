package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/cristianargotti/gh-board/internal/render"
	"github.com/spf13/cobra"
)

type writeIssueReader interface {
	IssueByID(context.Context, string) (domain.Issue, error)
}

type writeAction struct {
	step domain.Step
	run  func(context.Context, domain.Item) error
}

type writeSession struct {
	ctx     context.Context
	cmd     *cobra.Command
	deps    *Deps
	cfg     *domain.Config
	project domain.Project
	viewer  domain.Viewer
	plan    domain.Plan
	actions []writeAction
	items   map[string]domain.Item
	expect  map[string]map[string]string
	guarded bool
	// summary replaces the step descriptions as the plan description;
	// notes follow it.
	summary string
	notes   []string
}

func writeRun(cmd *cobra.Command, deps *Deps, build func(*writeSession) error) (err error) {
	s := &writeSession{
		ctx: cmd.Context(), cmd: cmd, deps: deps,
		items: map[string]domain.Item{}, expect: map[string]map[string]string{},
	}
	defer func() { err = s.writeAudit(err) }()
	if err = s.writeOpen(); err != nil {
		return err
	}
	if err = build(s); err != nil {
		return err
	}
	return s.writeExecute()
}

func (s *writeSession) writeOpen() error {
	if s.deps.Reader == nil || s.deps.Writer == nil || s.deps.Clock == nil || s.deps.Dirs.State == "" {
		return domain.Errorf(domain.ExitUsage, "writes need a reader, writer, clock and state directory")
	}
	if _, err := render.ParseFormat(s.deps.Flags.Format); err != nil {
		return err
	}
	cfg, err := writeConfig(s.deps)
	if err != nil {
		return err
	}
	s.cfg = cfg
	s.viewer, err = s.deps.Reader.Viewer(s.ctx)
	if err != nil {
		return writeAPI(err)
	}
	if s.viewer.Shadowed() {
		_, _ = fmt.Fprintf(s.cmd.ErrOrStderr(), "Warning: %s shadows the gh keyring token.\n", s.viewer.TokenSource)
	}
	s.project, err = s.deps.Reader.DiscoverProject(s.ctx, cfg.Project)
	if err != nil {
		return writeAPI(err)
	}
	if s.project.ViewerRole != domain.RoleWriter && s.project.ViewerRole != domain.RoleAdmin {
		return domain.Errorf(domain.ExitPolicy, "viewer cannot write to project %s", cfg.Project)
	}
	return s.writeInitPlan()
}

func writeConfig(deps *Deps) (*domain.Config, error) {
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
	cfg := domain.Config{}
	if deps.Config.Config != nil {
		cfg = *deps.Config.Config
	}
	if !ref.IsZero() {
		cfg.Project = ref
	}
	if cfg.Project.IsZero() {
		return nil, readNoProject(deps.Config)
	}
	return &cfg, nil
}

func (s *writeSession) writeInitPlan() error {
	now := s.deps.Now()
	id, err := plan.NewID(now)
	if err != nil {
		return err
	}
	s.plan = domain.Plan{
		ID: id, KitVersion: s.deps.Version, CreatedAt: now,
		ExpiresAt: now.Add(domain.PlanExpiry), Actor: s.viewer.Login, Host: s.viewer.Host,
		Project: s.cfg.Project, ProjectID: s.project.NodeID, Command: s.cmd.CommandPath(),
	}
	return nil
}

// writeDescription words the plan for the team: the summary a verb set,
// or the distinct step descriptions in order, then the notes.
func (s *writeSession) writeDescription() string {
	parts := []string{}
	if s.summary != "" {
		parts = append(parts, s.summary)
	} else {
		seen := map[string]bool{}
		for _, action := range s.actions {
			if d := action.step.Description; d != "" && !seen[d] {
				seen[d] = true
				parts = append(parts, d)
			}
		}
	}
	parts = append(parts, s.notes...)
	if len(parts) == 0 {
		return "Atualização de trabalho no quadro."
	}
	return strings.Join(parts, " ")
}

func (s *writeSession) writePermission(owner, repo string) error {
	if owner == "" || repo == "" {
		return domain.Errorf(domain.ExitUsage, "target repository is missing")
	}
	perm, err := s.deps.Reader.ViewerPermission(s.ctx, owner, repo)
	if err != nil {
		return writeAPI(err)
	}
	if !perm.CanWrite() {
		return domain.Errorf(domain.ExitPolicy, "viewer cannot write to repository %s/%s", owner, repo)
	}
	return nil
}

func (s *writeSession) writeAudit(err error) error {
	if s.deps.Dirs.State == "" || s.deps.Clock == nil {
		return err
	}
	e := audit.Entry{
		At: s.deps.Now(), Actor: s.viewer.Login, Host: s.viewer.Host,
		Project: s.project.Ref, Command: s.cmd.CommandPath(), Reason: s.deps.Flags.Reason,
		Targets: s.writeAuditTargets(), Items: s.writeAuditItems(), PlanID: s.plan.ID,
		Result: domain.CodeOf(err).String(), DryRun: s.deps.Flags.DryRun,
	}
	if err != nil {
		e.Error = render.Sanitize(err.Error(), 0)
	}
	return errors.Join(err, audit.Append(s.deps.Dirs.State, e))
}

func writeAPI(err error) error {
	if err == nil {
		return nil
	}
	var coded *domain.Error
	if errors.As(err, &coded) {
		return err
	}
	for _, sentinel := range []error{
		domain.ErrNotFound, domain.ErrAmbiguous, domain.ErrUsage,
		domain.ErrPolicy, domain.ErrDrift, domain.ErrPlanRequired, domain.ErrApplyRefused,
	} {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	return domain.NewError(domain.ExitAPI, err)
}

func writeUnavailable(name string) error {
	return domain.Errorf(domain.ExitUsage, "%s unavailable: not mapped in board.yml", name)
}

func writePair(text string) (string, string, error) {
	field, value, ok := strings.Cut(text, "=")
	if !ok || strings.TrimSpace(field) == "" {
		return "", "", domain.Errorf(domain.ExitUsage, "expected field=value, got %q", text)
	}
	return strings.TrimSpace(field), value, nil
}

// writeAuditItems names the targets as owner/repo#n, the plan's steps
// first, then the items the command resolved before any step existed.
func (s *writeSession) writeAuditItems() []string {
	if items := s.plan.Items(); len(items) != 0 {
		return items
	}
	refs := make([]string, 0, len(s.items))
	for _, it := range s.items {
		refs = append(refs, it.Issue.Ref())
	}
	sort.Strings(refs)
	return refs
}

func (s *writeSession) writeAuditTargets() []string {
	targets := s.plan.Targets()
	if len(targets) != 0 {
		return targets
	}
	for id := range s.items {
		targets = append(targets, id)
	}
	sort.Strings(targets)
	return targets
}
