package plan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

// execCase runs one step through the executor over the fixtures.
type execCase struct {
	name       string
	step       domain.Step
	reader     func(r *fakeReader)
	writer     func(w *fakeWriter)
	bare       bool
	snapshot   func(s *plan.Snapshot)
	created    map[int]string
	wantCalls  []string
	wantStatus string
	wantNode   string
	wantErr    error
	wantText   string
}

func runExecCases(t *testing.T, cases []execCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, w := newReader(), newWriter()
			if tc.reader != nil {
				tc.reader(r)
			}
			if tc.writer != nil {
				tc.writer(w)
			}
			snap := plan.Snapshot{Project: project, Items: map[string]domain.Item{"I_1": r.items["I_1"]}}
			if tc.snapshot != nil {
				tc.snapshot(&snap)
			}
			var reader domain.ProjectReader = r
			if tc.bare {
				reader = bareReader{r}
			}
			exec := plan.NewExecutor(newPorts(reader, w), newPlan(t, closeStep()), snap, viewer, now, tc.created)
			out, err := exec.Execute(context.Background(), tc.step)
			checkExec(t, tc, out, err, w.calls)
		})
	}
}

// checkExec compares the outcome, the error and the writer calls.
func checkExec(t *testing.T, tc execCase, out plan.Outcome, err error, calls []string) {
	t.Helper()
	if tc.wantErr != nil || tc.wantText != "" {
		if err == nil {
			t.Fatalf("expected an error, got outcome %+v", out)
		}
		if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
			t.Fatalf("error = %v, want %v", err, tc.wantErr)
		}
		if tc.wantText != "" && !contains(err.Error(), tc.wantText) {
			t.Fatalf("error = %v, want text %q", err, tc.wantText)
		}
		return
	}
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	equalCalls(t, calls, tc.wantCalls)
	if out.Status != tc.wantStatus || out.NodeID != tc.wantNode {
		t.Fatalf("outcome = %+v, want %s %q", out, tc.wantStatus, tc.wantNode)
	}
}

// withTarget swaps the target node id of a step.
func withTarget(s domain.Step, nodeID string) domain.Step {
	s.Target.NodeID = nodeID
	return s
}

func TestKnownOperation(t *testing.T) {
	for _, op := range []domain.Operation{domain.OpCloseIssue, domain.OpCopyProject, domain.OpLinkRepository} {
		if !plan.KnownOperation(op) {
			t.Fatalf("%s must be known", op)
		}
	}
	if plan.KnownOperation("delete_issue") {
		t.Fatal("delete_issue must be unknown")
	}
}
