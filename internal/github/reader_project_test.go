package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestProjectByID(t *testing.T) {
	r, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	p, err := a.ProjectByID(ctx, acmeProjectID)
	if err != nil || p.NodeID != acmeProjectID || p.Ref != acmeBoard || p.Title != "Time Produto" || p.ViewerRole != domain.RoleWriter {
		t.Fatalf("project: %v, %+v", err, p)
	}
	if len(p.Fields) != 32 || len(p.Views) != 9 || !p.DiscoveredAt.Equal(testNow) {
		t.Fatalf("schema: %d fields, %d views, %s", len(p.Fields), len(p.Views), p.DiscoveredAt)
	}
	if _, err := a.DiscoverProject(ctx, acmeBoard); err != nil || r.count() != 1 {
		t.Fatalf("the discovery by id must fill the cache: %v, requests %d", err, r.count())
	}
	own, err := a.ProjectByID(ctx, sandboxProjectID)
	if err != nil || own.Ref != sandbox || own.ViewerRole != domain.RoleAdmin {
		t.Fatalf("sandbox project: %v, %+v", err, own)
	}
	if _, err := a.ProjectByID(ctx, issue111ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("an issue node is not a project: %v", err)
	}
	before := r.count()
	if _, err := a.ProjectByID(ctx, ""); !errors.Is(err, domain.ErrUsage) || r.count() != before {
		t.Fatalf("an empty id is refused before any request: %v", err)
	}
}
