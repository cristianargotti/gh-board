package plan_test

import (
	"context"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestApplyLateDriftPreventsEarlierWrite(t *testing.T) {
	opts, _ := applyOptions(t)
	r, w := newReader(), newWriter()
	second := withTarget(step(domain.OpSetFieldValue, "Estimate", "1"), "I_second")
	it := issueItem()
	it.Issue.NodeID = second.Target.NodeID
	it.Values["Estimate"] = domain.FieldValue{Value: "9"}
	r.items[it.Issue.NodeID] = it
	p := newPlan(t, closeStep(), second)
	_, err := plan.Apply(context.Background(), newPorts(r, w), p, opts)
	if domain.CodeOf(err) != domain.ExitDrift {
		t.Fatal(err)
	}
	equalCalls(t, r.calls, []string{"DiscoverProject acme/7", "GetItem I_1", "GetItem I_second"})
	equalCalls(t, w.calls, nil)
	if len(journalOf(t, opts.StateDir, p.ID)) != 0 || auditOf(t, opts.StateDir)[0].Result != "drift" {
		t.Fatal("drift was not recorded without executing steps")
	}
}

func TestApplyRefusesIncompleteJournalBinding(t *testing.T) {
	opts, _ := applyOptions(t)
	r, w := newReader(), newWriter()
	p := creationPlan(t)
	if err := plan.NewJournal(opts.StateDir).Append(plan.JournalEntry{PlanID: p.ID, Step: 0, Status: plan.StatusDone}); err != nil {
		t.Fatal(err)
	}
	_, err := plan.Apply(context.Background(), newPorts(r, w), p, opts)
	if domain.CodeOf(err) != domain.ExitApplyRefused {
		t.Fatal(err)
	}
	equalCalls(t, r.calls, nil)
	equalCalls(t, w.calls, nil)
}

func TestApplyDryRunValidatesRefusals(t *testing.T) {
	for _, tc := range refusals[1:] {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.opts
			tc.opts = func(opts *plan.ApplyOptions) {
				opts.DryRun = true
				if original != nil {
					original(opts)
				}
			}
			w, _, err := refusedApply(t, tc)
			if domain.CodeOf(err) != domain.ExitApplyRefused || len(w.calls) != 0 {
				t.Fatalf("dry run refusal = %v, calls %v", err, w.calls)
			}
		})
	}
}

func TestApplyDryRunChecksAndPrintsPendingSteps(t *testing.T) {
	opts, out := applyOptions(t)
	opts.DryRun, opts.Interactive = true, false
	r, w := newReader(), newWriter()
	p := newPlan(t, closeStep(), step(domain.OpSetFieldValue, "Status", "Todo"))
	result, err := plan.Apply(context.Background(), newPorts(r, w), p, opts)
	if err != nil || !result.DryRun || len(result.Done)+len(result.Skipped) != 0 {
		t.Fatalf("dry run = %+v, %v", result, err)
	}
	equalCalls(t, r.calls, []string{"DiscoverProject acme/7", "GetItem I_1"})
	equalCalls(t, w.calls, nil)
	for _, want := range []string{"1. close_issue", "2. set_field_value", "would run", "already in place"} {
		if !contains(out.String(), want) {
			t.Fatalf("output lacks %q: %s", want, out)
		}
	}
	if len(journalOf(t, opts.StateDir, p.ID)) != 0 || auditOf(t, opts.StateDir)[0].Result != "dry-run" {
		t.Fatal("dry run wrote steps or lost its audit")
	}
}
