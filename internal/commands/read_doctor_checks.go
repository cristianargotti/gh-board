package commands

import (
	"context"
	"fmt"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readDoctorConfigCheck reports which file is in use and its hash (F6).
// A file that does not load is a finding; the checks go on in generic
// mode.
func readDoctorConfigCheck(deps *Deps, rep *readDoctorReport) *domain.Config {
	loaded, cfg, err := readLoad(deps)
	if err != nil {
		rep.Config.Error = err.Error()
		rep.problem("configuration: " + err.Error())
		return &domain.Config{Version: config.SupportedVersion}
	}
	rep.Config = readDoctorConfig{
		Source: string(loaded.Source), Path: loaded.Path, SHA256: loaded.Hash, Generic: loaded.Generic(),
		Repository: cfg.Repository, DefaultPath: loaded.Default.Path, Candidates: loaded.Candidates,
	}
	if !cfg.Project.IsZero() {
		rep.Config.Project = cfg.Project.String()
	}
	if !loaded.Default.IsZero() {
		rep.Config.DefaultProject = loaded.Default.Project.String()
	}
	return cfg
}

func readDoctorEnvironmentOf(env config.Environment, dirs config.Dirs) readDoctorEnvironment {
	return readDoctorEnvironment{
		ConfigPath: env.ConfigPath, Home: env.Home, GHToken: env.GHToken, GitHubToken: env.GitHubToken,
		Shadow: env.TokenShadow(), ConfigDir: dirs.Config, StateDir: dirs.State, CacheDir: dirs.Cache,
	}
}

// readDoctorIdentityCheck resolves the effective actor and reports when an
// environment token shadows the keyring (section 6.6).
func readDoctorIdentityCheck(ctx context.Context, deps *Deps, rep *readDoctorReport) {
	if deps.Reader == nil {
		rep.Identity.Error = readNoReader
		rep.problem("identity: " + rep.Identity.Error)
		return
	}
	viewer, err := deps.Reader.Viewer(ctx)
	if err != nil {
		rep.Identity.Error = "viewer unavailable: " + err.Error()
		rep.problem("identity: " + rep.Identity.Error)
		return
	}
	rep.Identity = readDoctorIdentity{
		Login: viewer.Login, Host: viewer.Host, TokenSource: viewer.TokenSource,
		Shadowed: viewer.Shadowed() || rep.Environment.Shadow != "",
	}
	if rep.Identity.Shadowed {
		rep.problem("identity: an environment token shadows the stored gh credentials")
	}
}

// readDoctorProjectCheck discovers the project; the discovered schema is
// what the mismatch check reads.
func readDoctorProjectCheck(ctx context.Context, deps *Deps, cfg *domain.Config, rep *readDoctorReport) *domain.Project {
	if cfg.Project.IsZero() {
		rep.Project.Error = readNoProjectText(deps.Config)
		rep.problem("project: " + rep.Project.Error)
		return nil
	}
	rep.Project.Ref = cfg.Project.String()
	if deps.Reader == nil {
		rep.Project.Error = readNoReader
		rep.problem("project: " + rep.Project.Error)
		return nil
	}
	project, err := deps.Reader.DiscoverProject(ctx, cfg.Project)
	if err != nil {
		rep.Project.Error = "unreachable: " + err.Error()
		rep.problem("project: " + rep.Project.Error)
		return nil
	}
	rep.Project.OK = true
	rep.Project.Title = render.Title(project.Title)
	rep.Project.ViewerRole = string(project.ViewerRole)
	rep.Project.ItemCount = project.ItemCount
	return &project
}

// readDoctorMismatchCheck validates every name of board.yml against the
// discovered schema and lists the commands each mismatch breaks.
func readDoctorMismatchCheck(ctx context.Context, deps *Deps, cfg *domain.Config, project *domain.Project, rep *readDoctorReport) {
	if project == nil || rep.Config.Generic {
		return
	}
	schema := config.Schema{Project: *project}
	if cfg.Repository != "" {
		if owner, name, err := domain.SplitRepository(cfg.Repository); err == nil {
			labels, err := deps.Reader.RepositoryLabels(ctx, owner, name)
			if err != nil {
				rep.Project.Notes = append(rep.Project.Notes, "labels not checked: "+err.Error())
			} else {
				schema.Labels = labels
			}
		}
	}
	types, err := deps.Reader.IssueTypes(ctx, project.Ref.Owner)
	if err != nil {
		rep.Project.Notes = append(rep.Project.Notes, "issue types not checked: "+err.Error())
	} else {
		schema.IssueTypes = types
	}
	readDoctorIssueFields(ctx, deps, project.Ref.Owner, &schema, rep)
	mismatches, err := config.ValidateSchema(cfg, schema)
	if err != nil {
		rep.problem("board.yml: " + err.Error())
		return
	}
	for _, m := range mismatches {
		rep.Mismatches = append(rep.Mismatches, readDoctorMismatch{Path: m.Path, Value: m.Value, Reason: m.Reason, Commands: m.Commands})
		rep.problem(fmt.Sprintf("board.yml %s: %s", m.Path, m.Reason))
	}
}

// readDoctorRepositoryCheck reports the viewer's permission on the
// repository that receives new issues.
func readDoctorRepositoryCheck(ctx context.Context, deps *Deps, cfg *domain.Config, rep *readDoctorReport) {
	if cfg.Repository == "" {
		rep.Repository.Error = "repository not configured: new and the short form #n need it"
		return
	}
	rep.Repository.Name = cfg.Repository
	owner, name, err := domain.SplitRepository(cfg.Repository)
	if err != nil {
		rep.Repository.Error = err.Error()
		rep.problem("repository: " + err.Error())
		return
	}
	if deps.Reader == nil {
		rep.Repository.Error = readNoReader
		return
	}
	perm, err := deps.Reader.ViewerPermission(ctx, owner, name)
	if err != nil {
		rep.Repository.Error = "permission unavailable: " + err.Error()
		rep.problem("repository: " + rep.Repository.Error)
		return
	}
	rep.Repository.Permission = string(perm)
	rep.Repository.CanWrite = perm.CanWrite()
	if !perm.CanWrite() {
		rep.problem(fmt.Sprintf("repository: no write permission on %s (%s)", cfg.Repository, perm))
	}
}

// readIssueFieldLister is the narrow port that lists the organization
// issue fields, so doctor can check the dates declared with the
// issue_fields source; a reader without it skips that check.
type readIssueFieldLister interface {
	IssueFields(ctx context.Context, owner string) ([]domain.Field, error)
}

func readDoctorIssueFields(ctx context.Context, deps *Deps, owner string, schema *config.Schema, rep *readDoctorReport) {
	lister, ok := deps.Reader.(readIssueFieldLister)
	if !ok {
		return
	}
	fields, err := lister.IssueFields(ctx, owner)
	if err != nil {
		rep.Project.Notes = append(rep.Project.Notes, "issue fields not checked: "+err.Error())
		return
	}
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	schema.IssueFields = names
}
