package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

func TestRepositoryLabels(t *testing.T) {
	r, a := newReplay(t, "acme", "errors")
	ctx := context.Background()
	labels, err := a.RepositoryLabels(ctx, "acme", "app")
	if err != nil || len(labels) != 45 || r.count() != 2 {
		t.Fatalf("labels: %v, %d labels over %d requests", err, len(labels), r.count())
	}
	if labels[0].Name != "Amazon Q development agent" || labels[0].ID == "" || labels[0].Color == "" {
		t.Fatalf("first label = %+v", labels[0])
	}
	if _, err := domain.ResolveLabel(labels, "BUG"); err != nil {
		t.Fatal(err)
	}
	if r.all()[1].Variables["after"] != "MjU" {
		t.Fatalf("second page cursor = %v", r.all()[1].Variables["after"])
	}
	_, err = a.RepositoryLabels(ctx, "acme", "repo-that-does-not-exist")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing repository: %v", err)
	}
}

func TestRepositoryMilestones(t *testing.T) {
	_, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	milestones, err := a.RepositoryMilestones(ctx, "acme", "app")
	if err != nil || len(milestones) != 8 {
		t.Fatalf("milestones: %v, %d", err, len(milestones))
	}
	if m := milestones[0]; m.Title != "Marco 2" || m.Number != 1 || m.State != "OPEN" || m.DueOn == nil || m.ID == "" {
		t.Fatalf("first milestone = %+v", m)
	}
	none, err := a.RepositoryMilestones(ctx, "membro1", "gh-board-sandbox")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no milestones is an empty list: %v, %v", err, none)
	}
}

func TestIssueTypes(t *testing.T) {
	_, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	types, err := a.IssueTypes(ctx, "acme")
	if err != nil || len(types) != 3 || types[0].Name != "Task" || types[2].Name != "Feature" || types[0].ID == "" {
		t.Fatalf("types: %v, %+v", err, types)
	}
	userTypes, err := a.IssueTypes(ctx, "membro1")
	if err != nil || userTypes == nil || len(userTypes) != 0 {
		t.Fatalf("a user owner has no issue types and no error: %v, %v", err, userTypes)
	}
}

func TestIssueComments(t *testing.T) {
	r, a := newReplay(t, "acme")
	ctx := context.Background()
	comments, err := a.IssueComments(ctx, "I_kud9Ny7kjDmOS1mJGdwQGO")
	if err != nil || len(comments) != 2 || r.count() != 2 {
		t.Fatalf("comments: %v, %d over %d requests", err, len(comments), r.count())
	}
	for _, c := range comments {
		if c.ID == "" || c.Body == "" || c.Author == "" || c.CreatedAt.IsZero() || c.URL == "" {
			t.Fatalf("comment = %+v", c)
		}
	}
	if !comments[0].CreatedAt.Before(comments[1].CreatedAt) {
		t.Fatal("comments must come oldest first")
	}
	if _, err := a.IssueComments(ctx, acmeProjectID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a node that is not an issue: %v", err)
	}
}

func TestViewerIdentity(t *testing.T) {
	r, a := newReplay(t, "acme")
	v, err := a.Viewer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Login != "membro1" || v.ID != "U_2JRxF9gBtX" || v.Host != "github.com" || v.TokenSource != "GH_TOKEN" || !v.Shadowed() {
		t.Fatalf("viewer = %+v", v)
	}
	if r.last().Auth != "token "+replayToken {
		t.Fatalf("the gh token must authenticate the request, got %q", r.last().Auth)
	}
}

func TestViewerTokenSourceFromGitHubToken(t *testing.T) {
	r, a := newReplay(t, "acme")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "actions-token")
	v, err := a.Viewer(context.Background())
	if err != nil || v.TokenSource != "GITHUB_TOKEN" || !v.Shadowed() {
		t.Fatalf("viewer = %+v, %v", v, err)
	}
	if r.last().Auth != "token actions-token" {
		t.Fatalf("auth = %q", r.last().Auth)
	}
}

func TestViewerUnauthorized(t *testing.T) {
	_, a := newReplay(t, "errors")
	_, err := a.Viewer(context.Background())
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) || !apiErr.Auth || apiErr.Status != 401 || !errors.Is(err, domain.ErrAPI) {
		t.Fatalf("expected a 401 APIError, got %v", err)
	}
}

var permissionFixtureCases = []struct {
	name  string
	dirs  []string
	owner string
	repo  string
	want  domain.Permission
	err   error
}{
	{"writer on the team repository", []string{"acme"}, "acme", "app", domain.PermissionWrite, nil},
	{"admin on the sandbox", []string{"sandbox"}, "membro1", "gh-board-sandbox", domain.PermissionAdmin, nil},
	{"missing repository", []string{"acme"}, "acme", "repo-that-does-not-exist", domain.PermissionNone, domain.ErrNotFound},
}

func TestViewerPermission(t *testing.T) {
	for _, tc := range permissionFixtureCases {
		t.Run(tc.name, func(t *testing.T) {
			_, a := newReplay(t, tc.dirs...)
			got, err := a.ViewerPermission(context.Background(), tc.owner, tc.repo)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("permission = %s, %v; want %s, %v", got, err, tc.want, tc.err)
			}
			if err == nil && !got.CanWrite() {
				t.Fatal("writers can write")
			}
		})
	}
}
