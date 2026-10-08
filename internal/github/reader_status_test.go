package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestListStatusUpdates(t *testing.T) {
	r, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	updates, err := a.ListStatusUpdates(ctx, acmeProjectID)
	if err != nil || len(updates) != 1 {
		t.Fatalf("updates: %v, %+v", err, updates)
	}
	u := updates[0]
	if u.ID == "" || u.Body == "" || u.Status != "ON_TRACK" || u.Creator != "membro1" || !u.CreatedAt.Equal(time.Date(2026, 9, 22, 15, 30, 29, 0, time.UTC)) {
		t.Fatalf("update = %+v", u)
	}
	if u.StartDate == nil || u.TargetDate == nil || u.StartDate.Format(domain.DateLayout) != "2026-09-22" || u.TargetDate.Format(domain.DateLayout) != "2026-10-02" {
		t.Fatalf("dates = %v %v", u.StartDate, u.TargetDate)
	}
	sandbox, err := a.ListStatusUpdates(ctx, sandboxProjectID)
	if err != nil || len(sandbox) != 1 || sandbox[0].Status != "ON_TRACK" {
		t.Fatalf("sandbox updates: %v, %+v", err, sandbox)
	}
	if _, err := a.ListStatusUpdates(ctx, issue111ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("an issue node has no status updates: %v", err)
	}
	before := r.count()
	if _, err := a.ListStatusUpdates(ctx, ""); !errors.Is(err, domain.ErrUsage) || r.count() != before {
		t.Fatalf("an empty id is refused before any request: %v", err)
	}
}
