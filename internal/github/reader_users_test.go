package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestAssignableUsers(t *testing.T) {
	r, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	users, err := a.AssignableUsers(ctx, "acme", "app")
	if err != nil || len(users) != 131 || r.count() != 2 {
		t.Fatalf("users: %v, %d users over %d requests", err, len(users), r.count())
	}
	if users[0].ID == "" || users[0].Login == "" || r.all()[1].Variables["after"] == nil {
		t.Fatalf("first user %+v, second page variables %v", users[0], r.all()[1].Variables)
	}
	if _, err := domain.ResolveUser(users, "membro17"); err != nil {
		t.Fatal(err)
	}
	one, err := a.AssignableUsers(ctx, "membro1", "gh-board-sandbox")
	if err != nil || len(one) != 1 || one[0].ID != sandboxViewerID || one[0].Login != "membro1" {
		t.Fatalf("sandbox users: %v, %+v", err, one)
	}
	if _, err := a.AssignableUsers(ctx, "acme", "repo-that-does-not-exist"); !errors.Is(err, domain.ErrAPI) {
		t.Fatalf("an unrecorded repository answers an API error: %v", err)
	}
	before := r.count()
	if _, err := a.AssignableUsers(ctx, "", "x"); !errors.Is(err, domain.ErrUsage) || r.count() != before {
		t.Fatalf("an empty owner is refused before any request: %v", err)
	}
}
