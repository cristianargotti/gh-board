package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestIssueTimelineWithFieldEvents(t *testing.T) {
	r, a := newReplay(t, "acme")
	tl, err := a.IssueTimeline(context.Background(), fieldsIssueID)
	if err != nil || !tl.Complete || tl.Reason != "" {
		t.Fatalf("timeline: %v, complete %v, reason %q", err, tl.Complete, tl.Reason)
	}
	if r.count() != 5 {
		t.Fatalf("four timeline pages and one item read, got %d requests", r.count())
	}
	if len(tl.Comments) != 2 || tl.Comments[0].Author != "membro1" || !tl.Comments[0].CreatedAt.Before(tl.Comments[1].CreatedAt) {
		t.Fatalf("comments = %+v", tl.Comments)
	}
	assertFieldEvents(t, tl.Decisions[:6])
	assertProjectDecisions(t, tl.Decisions[6:])
}

func assertFieldEvents(t *testing.T, events []domain.DecisionEvent) {
	t.Helper()
	first := events[0]
	if first.Field != "Start date" || first.Value != "2026-09-22" || first.Actor != "membro1" || first.ID == "" ||
		!first.CreatedAt.Equal(time.Date(2026, 9, 22, 15, 11, 16, 0, time.UTC)) {
		t.Fatalf("first event = %+v", first)
	}
	if option := events[2]; option.Field != "Priority" || option.Value != "High" {
		t.Fatalf("option event = %+v", option)
	}
	if last := events[5]; last.Field != "Target date" || last.Value != "2026-10-07" {
		t.Fatalf("last change = %+v", last)
	}
}

func assertProjectDecisions(t *testing.T, events []domain.DecisionEvent) {
	t.Helper()
	if len(events) != 6 {
		t.Fatalf("project decisions = %+v", events)
	}
	byField := map[string]domain.DecisionEvent{}
	for _, e := range events {
		byField[e.Field] = e
	}
	status := byField["Status"]
	if status.Value != "IN PROGRESS" || status.Actor != "membro1" || status.CreatedAt.IsZero() || status.ID == "" {
		t.Fatalf("status decision = %+v", status)
	}
	if _, ok := byField["Start date"]; ok {
		t.Fatal("a value without a creator is not a decision")
	}
}

func TestIssueTimelineCommentsOnly(t *testing.T) {
	r, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	tl, err := a.IssueTimeline(ctx, issue111ID)
	if err != nil || !tl.Complete || len(tl.Comments) != 2 || len(tl.Decisions) != 7 || r.count() != 2 {
		t.Fatalf("timeline: %v, %d comments, %d decisions, %d requests", err, len(tl.Comments), len(tl.Decisions), r.count())
	}
	one, err := a.IssueTimeline(ctx, sandboxIssueID)
	if err != nil || len(one.Comments) != 1 || one.Comments[0].Body == "" || one.Comments[0].URL == "" || len(one.Decisions) != 5 {
		t.Fatalf("sandbox timeline: %v, %+v", err, one)
	}
	if _, err := a.IssueTimeline(ctx, acmeProjectID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a project node has no timeline: %v", err)
	}
	before := r.count()
	if _, err := a.IssueTimeline(ctx, ""); !errors.Is(err, domain.ErrUsage) || r.count() != before {
		t.Fatalf("an empty id is refused before any request: %v", err)
	}
}

func TestIssueTimelineOddShapes(t *testing.T) {
	_, a := newReplay(t, "errors")
	ctx := context.Background()
	tl, err := a.IssueTimeline(ctx, "I_oddTimeline00000")
	if err != nil || !tl.Complete {
		t.Fatalf("timeline: %v, %+v", err, tl)
	}
	if len(tl.Comments) != 1 || tl.Comments[0].Author != "" {
		t.Fatalf("a comment of a removed author keeps an empty login: %+v", tl.Comments)
	}
	if len(tl.Decisions) != 1 || tl.Decisions[0].Value != "Medium, Low" || tl.Decisions[0].Actor != "" {
		t.Fatalf("an event without a field is dropped and options join: %+v", tl.Decisions)
	}
	if _, err := a.IssueTimeline(ctx, "I_orphanTimeline00"); !errors.Is(err, domain.ErrAPI) {
		t.Fatalf("an item read that fails is reported: %v", err)
	}
}
