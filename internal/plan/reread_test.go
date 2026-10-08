package plan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestReread(t *testing.T) {
	r := newReader()
	steps := []domain.Step{
		closeStep(),
		step(domain.OpAddProjectItem, "", ""),
		{Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: "I_missing"}, Field: "Status"},
		{Operation: domain.OpAddComment, Target: domain.Target{NodeID: "I_epic"}},
		{Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: plan.Ref(0)}},
		{Operation: domain.OpCreateLabel},
	}
	snap, err := plan.Reread(context.Background(), r, newPlan(t, closeStep()), steps)
	if err != nil {
		t.Fatalf("Reread: %v", err)
	}
	if snap.Project.NodeID != "PVT_1" || len(snap.Items) != 2 || snap.Items["I_1"].Issue.Number != 12 {
		t.Fatalf("snapshot = %+v", snap)
	}
	equalCalls(t, r.calls, []string{"DiscoverProject acme/7", "GetItem I_1", "GetItem I_missing", "GetItem I_epic"})
}

func TestRereadWithoutProject(t *testing.T) {
	r := newReader()
	p := newPlan(t, closeStep())
	p.Project = domain.ProjectRef{}
	snap, err := plan.Reread(context.Background(), r, p, p.Steps)
	if err != nil || snap.Project.NodeID != "" || len(snap.Items) != 1 {
		t.Fatalf("Reread = %+v, %v", snap, err)
	}
	equalCalls(t, r.calls, []string{"GetItem I_1"})
}

var rereadFailures = []struct {
	name   string
	method string
}{
	{name: "discovery fails", method: "DiscoverProject"},
	{name: "item read fails", method: "GetItem"},
}

func TestRereadFailures(t *testing.T) {
	for _, tc := range rereadFailures {
		t.Run(tc.name, func(t *testing.T) {
			r := newReader()
			r.fail[tc.method] = domain.ErrAPI
			p := newPlan(t, closeStep())
			if _, err := plan.Reread(context.Background(), r, p, p.Steps); !errors.Is(err, domain.ErrAPI) {
				t.Fatalf("expected ErrAPI, got %v", err)
			}
		})
	}
}

var rereadCreatedCases = []struct {
	name    string
	created bool
	bare    bool
	fail    error
	found   bool
	err     error
}{
	{name: "created issue off the board", created: true, found: true},
	{name: "not created stays a drift", created: false, found: false},
	{name: "reader without the port", created: true, bare: true, found: false},
	{name: "issue read fails", created: true, fail: domain.ErrAPI, err: domain.ErrAPI},
}

func TestRereadCreated(t *testing.T) {
	for _, tc := range rereadCreatedCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReader()
			r.issues = map[string]domain.Issue{"I_new": {NodeID: "I_new", Owner: "acme", Repo: "app", Number: 40, State: domain.IssueOpen}}
			r.fail["IssueByID"] = tc.fail
			var reader plan.TargetReader = r
			if tc.bare {
				reader = bareReader{r}
			}
			steps := []domain.Step{{Operation: domain.OpCloseIssue, Target: domain.Target{NodeID: "I_new"}, Before: "OPEN", After: "CLOSED"}}
			created := map[string]bool{"I_new": tc.created}
			snap, err := plan.RereadCreated(context.Background(), reader, newPlan(t, closeStep()), steps, created)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if _, ok := snap.Items["I_new"]; ok != tc.found {
				t.Fatalf("found %v, want %v: %+v", ok, tc.found, snap.Items)
			}
			if tc.found && (snap.Items["I_new"].Issue.Number != 40 || snap.Items["I_new"].ProjectItemID != "") {
				t.Fatalf("item = %+v", snap.Items["I_new"])
			}
		})
	}
}
