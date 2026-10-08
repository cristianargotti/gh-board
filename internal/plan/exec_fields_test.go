package plan_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var fieldCases = []execCase{
	{name: "single select", step: step(domain.OpSetFieldValue, "Status", "done"), wantCalls: []string{"UpdateItemFieldValue PVT_1 PVTI_1 F_status option=O_done"}, wantStatus: plan.StatusDone},
	{name: "unknown option", step: step(domain.OpSetFieldValue, "Status", "Blocked"), wantErr: domain.ErrNotFound},
	{name: "iteration", step: step(domain.OpSetFieldValue, "Sprint", "sprint 1"), wantCalls: []string{"UpdateItemFieldValue PVT_1 PVTI_1 F_sprint iteration=IT_1"}, wantStatus: plan.StatusDone},
	{name: "unknown iteration", step: step(domain.OpSetFieldValue, "Sprint", "Sprint 9"), wantErr: domain.ErrNotFound},
	{name: "number", step: step(domain.OpSetFieldValue, "Estimate", "2.5"), wantCalls: []string{"UpdateItemFieldValue PVT_1 PVTI_1 F_est number=2.5"}, wantStatus: plan.StatusDone},
	{name: "bad number", step: step(domain.OpSetFieldValue, "Estimate", "two"), wantErr: domain.ErrUsage},
	{name: "date", step: step(domain.OpSetFieldValue, "Start", "2026-10-10"), wantCalls: []string{"UpdateItemFieldValue PVT_1 PVTI_1 F_start date=2026-10-10"}, wantStatus: plan.StatusDone},
	{name: "bad date", step: step(domain.OpSetFieldValue, "Start", "10/10/2026"), wantErr: domain.ErrUsage},
	{name: "text", step: step(domain.OpSetFieldValue, "Notes", "hello"), wantCalls: []string{"UpdateItemFieldValue PVT_1 PVTI_1 F_notes text=hello"}, wantStatus: plan.StatusDone},
	{name: "unsupported type", step: step(domain.OpSetFieldValue, "Title", "x"), wantErr: domain.ErrUsage, wantText: "cannot be set"},
	{name: "unknown field", step: step(domain.OpSetFieldValue, "Lane", "x"), wantErr: domain.ErrNotFound},
	{name: "item id from snapshot", step: fieldWithoutItemID(), wantCalls: []string{"UpdateItemFieldValue PVT_1 PVTI_1 F_status option=O_done"}, wantStatus: plan.StatusDone},
	{name: "write fails", step: step(domain.OpSetFieldValue, "Status", "Done"), writer: func(w *fakeWriter) { w.fail["UpdateItemFieldValue"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	{name: "issue field update", step: step(domain.OpSetIssueFieldValue, "Start date", "2026-11-01"), wantCalls: []string{"UpdateIssueFieldValue I_1 IF_start 2026-11-01"}, wantStatus: plan.StatusDone},
	{name: "issue field first value", step: step(domain.OpSetIssueFieldValue, "Target date", "2026-11-01"), wantCalls: []string{"SetIssueFieldValue I_1 IF_target 2026-11-01"}, wantStatus: plan.StatusDone},
	{name: "issue field unknown", step: step(domain.OpSetIssueFieldValue, "Due", "2026-11-01"), wantErr: domain.ErrNotFound},
	{name: "issue field without resolver", step: step(domain.OpSetIssueFieldValue, "Target date", "2026-11-01"), bare: true, wantErr: domain.ErrNotImplemented},
	{name: "issue field empty target", step: withTarget(step(domain.OpSetIssueFieldValue, "Start date", "x"), ""), wantErr: domain.ErrUsage},
}

// fieldWithoutItemID leaves the project item to the re-read snapshot.
func fieldWithoutItemID() domain.Step {
	s := step(domain.OpSetFieldValue, "Status", "Done")
	s.Target.ProjectItemID = ""
	return s
}

func TestExecuteFieldOperations(t *testing.T) {
	runExecCases(t, fieldCases)
}
