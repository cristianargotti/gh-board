package plan_test

import (
	"context"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var recoveryCases = []struct {
	name    string
	value   string
	missing bool
	want    domain.ExitCode
}{
	{name: "concurrent estimate", value: "9", want: domain.ExitDrift},
	{name: "concurrent desired value", value: "1", want: domain.ExitDrift},
	{name: "missing target", missing: true, want: domain.ExitDrift},
	{name: "unchanged estimate", want: domain.ExitOK},
}

func creationPlan(t *testing.T) domain.Plan {
	t.Helper()
	return newPlan(t,
		domain.Step{Operation: domain.OpCreateIssue, After: payload(t, domain.CreateIssueInput{RepositoryID: "R_1", Title: "Nova"})},
		domain.Step{Operation: domain.OpAddProjectItem, Target: domain.Target{NodeID: plan.Ref(0)}},
		domain.Step{Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: plan.Ref(0), ProjectItemID: plan.Ref(1)}, Field: "Estimate", After: "1"},
	)
}

func TestApplyRechecksJournalCreatedTargets(t *testing.T) {
	for _, tc := range recoveryCases {
		t.Run(tc.name, func(t *testing.T) { checkCreatedRecovery(t, tc.value, tc.missing, tc.want) })
	}
}

func checkCreatedRecovery(t *testing.T, value string, missing bool, want domain.ExitCode) {
	t.Helper()
	opts, out := applyOptions(t)
	r, w := newReader(), newWriter()
	p := creationPlan(t)
	w.fail["UpdateItemFieldValue"] = domain.ErrAPI
	if _, err := plan.Apply(context.Background(), newPorts(r, w), p, opts); domain.CodeOf(err) != domain.ExitAPI {
		t.Fatalf("initial apply: %v", err)
	}
	if !missing {
		r.items[w.issue.NodeID] = domain.Item{Issue: w.issue, ProjectItemID: w.itemID, Values: map[string]domain.FieldValue{"Estimate": {Value: value}}}
	}
	r.calls, w.calls, w.fail = nil, nil, map[string]error{}
	_, err := plan.Apply(context.Background(), newPorts(r, w), p, opts)
	if domain.CodeOf(err) != want {
		t.Fatalf("resume exit = %d, want %d: %v; calls %v", domain.CodeOf(err), want, err, w.calls)
	}
	wantCalls := []string{"DiscoverProject acme/7", "GetItem I_new"}
	if missing {
		// An issue the board does not hold is read off the board before it
		// counts as drift.
		wantCalls = append(wantCalls, "IssueByID I_new")
	}
	equalCalls(t, r.calls, wantCalls)
	checkRecoveryWrites(t, w, journalOf(t, opts.StateDir, p.ID), want)
	if want == domain.ExitDrift && (!contains(out.String(), "Drift detected") || auditOf(t, opts.StateDir)[1].Result != "drift") {
		t.Fatalf("missing drift output/audit: %s", out)
	}
	if p.Steps[2].Target.NodeID != plan.Ref(0) || p.Verify() != nil {
		t.Fatal("recovery modified the immutable plan")
	}
}

func checkRecoveryWrites(t *testing.T, w *fakeWriter, entries []plan.JournalEntry, want domain.ExitCode) {
	t.Helper()
	if want == domain.ExitDrift {
		equalCalls(t, w.calls, nil)
		if len(entries) != 3 || entries[2].Status != plan.StatusFailed {
			t.Fatalf("drift changed the journal: %+v", entries)
		}
		return
	}
	equalCalls(t, w.calls, []string{"UpdateItemFieldValue PVT_1 PVTI_new F_est number=1"})
	if len(entries) != 4 || entries[3].Status != plan.StatusDone || entries[3].Target.NodeID != w.issue.NodeID {
		t.Fatalf("recovery journal = %+v", entries)
	}
}
