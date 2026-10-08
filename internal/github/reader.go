package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// permissionTriagePlus is the repository permission GitHub added above
// triage; the domain folds it into triage.
const permissionTriagePlus = "TRIAGE_PLUS"

// DiscoverProject reads the board schema (section 5.1), from the cache
// when it is younger than the TTL, otherwise from the API.
func (a *Adapter) DiscoverProject(ctx context.Context, ref domain.ProjectRef) (domain.Project, error) {
	conn, err := a.connect()
	if err != nil {
		return domain.Project{}, err
	}
	if p, ok := a.readCache(conn.host, ref); ok {
		return p, nil
	}
	return a.RefreshProject(ctx, ref)
}

// RefreshProject discovers the board again, ignoring the cache, and stores
// the result: schema --refresh and a write that failed on an unknown
// option both call it.
func (a *Adapter) RefreshProject(ctx context.Context, ref domain.ProjectRef) (domain.Project, error) {
	conn, err := a.connect()
	if err != nil {
		return domain.Project{}, err
	}
	var out discoverResponse
	err = a.run(ctx, "discover_project", map[string]any{varOwner: ref.Owner, varNumber: ref.Number}, &out)
	if err != nil && !tolerated(err, "organization", "user") {
		return domain.Project{}, err
	}
	raw := out.project()
	if raw == nil {
		if err != nil {
			return domain.Project{}, err
		}
		return domain.Project{}, notFound("discover_project", fmt.Sprintf("project %s", ref))
	}
	p := toProject(raw, out.Viewer.Login, a.clock.Now())
	// A cache directory that cannot be written must not break a read.
	_ = a.writeCache(conn.host, p)
	return p, nil
}

// tolerated reports whether the error is only NOT_FOUND under the paths,
// which happens on the owner kind that does not exist.
func tolerated(err error, paths ...string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.OnlyNotFoundAt(paths...)
}

// Viewer resolves the effective identity and its token source.
func (a *Adapter) Viewer(ctx context.Context) (domain.Viewer, error) {
	conn, err := a.connect()
	if err != nil {
		return domain.Viewer{}, err
	}
	var out struct {
		Viewer rawUser `json:"viewer"`
	}
	if err := a.run(ctx, "viewer", nil, &out); err != nil {
		return domain.Viewer{}, err
	}
	return domain.Viewer{
		Login:       out.Viewer.Login,
		ID:          out.Viewer.ID,
		Host:        conn.host,
		TokenSource: conn.tokenSource,
	}, nil
}

// ViewerPermission reports the viewer's permission on a repository.
func (a *Adapter) ViewerPermission(ctx context.Context, owner, repo string) (domain.Permission, error) {
	var out struct {
		Repository *struct {
			ViewerPermission string `json:"viewerPermission"`
		} `json:"repository"`
	}
	if err := a.run(ctx, "viewer_permission", map[string]any{varOwner: owner, varName: repo}, &out); err != nil {
		return domain.PermissionNone, err
	}
	if out.Repository == nil {
		return domain.PermissionNone, notFound("viewer_permission", "repository "+owner+"/"+repo)
	}
	return toPermission(out.Repository.ViewerPermission), nil
}

func toPermission(raw string) domain.Permission {
	switch p := domain.Permission(raw); p {
	case domain.PermissionAdmin, domain.PermissionMaintain, domain.PermissionWrite,
		domain.PermissionTriage, domain.PermissionRead:
		return p
	case permissionTriagePlus:
		return domain.PermissionTriage
	default:
		return domain.PermissionNone
	}
}
