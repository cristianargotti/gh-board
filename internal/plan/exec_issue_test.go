package plan_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var issueCases = []execCase{
	{name: "close", step: closeStep(), wantCalls: []string{"CloseIssue I_1"}, wantStatus: plan.StatusDone},
	{name: "reopen", step: step(domain.OpReopenIssue, "", "OPEN"), wantCalls: []string{"ReopenIssue I_1"}, wantStatus: plan.StatusDone},
	{name: "close fails", step: closeStep(), writer: func(w *fakeWriter) { w.fail["CloseIssue"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	{name: "empty target", step: withTarget(closeStep(), ""), wantErr: domain.ErrUsage, wantText: "no target"},
	{name: "dangling reference", step: withTarget(closeStep(), plan.Ref(4)), wantErr: domain.ErrUsage, wantText: "created nothing"},
	{name: "resolved reference", step: withTarget(closeStep(), plan.Ref(0)), created: map[int]string{0: "I_new"}, wantCalls: []string{"CloseIssue I_new"}, wantStatus: plan.StatusDone},
	{name: "unknown operation", step: domain.Step{Operation: "delete_issue", Target: target()}, wantErr: domain.ErrUsage},
	{name: "update title", step: step(domain.OpUpdateIssue, domain.UpdateFieldTitle, "New title"), wantCalls: []string{"UpdateIssue I_1 New title nil nil"}, wantStatus: plan.StatusDone},
	{name: "update body", step: step(domain.OpUpdateIssue, domain.UpdateFieldBody, "Body"), wantCalls: []string{"UpdateIssue I_1 nil Body nil"}, wantStatus: plan.StatusDone},
	{name: "update milestone", step: step(domain.OpUpdateIssue, domain.UpdateFieldMilestone, "v1"), wantCalls: []string{"UpdateIssue I_1 nil nil M_1"}, wantStatus: plan.StatusDone},
	{name: "update milestone unknown", step: step(domain.OpUpdateIssue, domain.UpdateFieldMilestone, "v9"), wantErr: domain.ErrNotFound},
	{name: "update milestone empty", step: step(domain.OpUpdateIssue, domain.UpdateFieldMilestone, ""), wantErr: domain.ErrUsage},
	{name: "update milestone read fails", step: step(domain.OpUpdateIssue, domain.UpdateFieldMilestone, "v1"), reader: func(r *fakeReader) { r.fail["RepositoryMilestones"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	{name: "update unknown field", step: step(domain.OpUpdateIssue, "state", "CLOSED"), wantErr: domain.ErrUsage},
	{name: "set issue type", step: step(domain.OpSetIssueType, "", "feature"), wantCalls: []string{"UpdateIssueType I_1 IT_feature"}, wantStatus: plan.StatusDone},
	{name: "set issue type unknown", step: step(domain.OpSetIssueType, "", "Bug"), wantErr: domain.ErrNotFound},
	{name: "set issue type read fails", step: step(domain.OpSetIssueType, "", "Task"), reader: func(r *fakeReader) { r.fail["IssueTypes"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	{name: "add sub issue by full ref", step: step(domain.OpAddSubIssue, "", "acme/repo#5"), wantCalls: []string{"AddSubIssue I_epic I_1"}, wantStatus: plan.StatusDone},
	{name: "add sub issue by number", step: step(domain.OpAddSubIssue, "", "#5"), wantCalls: []string{"AddSubIssue I_epic I_1"}, wantStatus: plan.StatusDone},
	{name: "add sub issue by node id", step: step(domain.OpAddSubIssue, "", "I_kwDOAbc123"), reader: func(r *fakeReader) { r.items["I_kwDOAbc123"] = epicItem() }, wantCalls: []string{"AddSubIssue I_epic I_1"}, wantStatus: plan.StatusDone},
	{name: "add sub issue malformed parent", step: step(domain.OpAddSubIssue, "", "???"), wantErr: domain.ErrUsage},
	{name: "add sub issue missing parent", step: step(domain.OpAddSubIssue, "", "acme/repo#77"), wantErr: domain.ErrNotFound},
	{name: "unarchive", step: step(domain.OpUnarchiveItem, "", domain.ActiveValue), wantCalls: []string{"UnarchiveItem PVT_1 PVTI_1"}, wantStatus: plan.StatusDone},
	{name: "unarchive without item id", step: unarchiveWithoutItem(), wantErr: domain.ErrUsage, wantText: "no project item"},
	{name: "unarchive dangling item reference", step: unarchiveWithItem(plan.Ref(7)), wantErr: domain.ErrUsage},
	{name: "add project item already there", step: step(domain.OpAddProjectItem, "", ""), wantStatus: plan.StatusSkipped, wantNode: "PVTI_1"},
	{name: "add project item", step: withTarget(step(domain.OpAddProjectItem, "", ""), "I_9"), wantCalls: []string{"AddProjectItem PVT_1 I_9"}, wantStatus: plan.StatusDone, wantNode: "PVTI_new"},
	{name: "add project item fails", step: withTarget(step(domain.OpAddProjectItem, "", ""), "I_9"), writer: func(w *fakeWriter) { w.fail["AddProjectItem"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
}

// unarchiveWithoutItem targets an item the snapshot does not hold.
func unarchiveWithoutItem() domain.Step {
	s := withTarget(step(domain.OpUnarchiveItem, "", domain.ActiveValue), "I_9")
	s.Target.ProjectItemID = ""
	return s
}

// unarchiveWithItem names the project item explicitly.
func unarchiveWithItem(itemID string) domain.Step {
	s := step(domain.OpUnarchiveItem, "", domain.ActiveValue)
	s.Target.ProjectItemID = itemID
	return s
}

func TestExecuteIssueOperations(t *testing.T) {
	runExecCases(t, issueCases)
}
