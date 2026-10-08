package plan_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

// unassigned replaces the fixture issue with one without assignees.
func unassigned(r *fakeReader) {
	it := issueItem()
	it.Issue.Assignees = nil
	r.items["I_1"] = it
}

var setCases = []execCase{
	{name: "add assignee", step: step(domain.OpAddAssignees, "", "hubot,octocat"), wantCalls: []string{"AddAssignees I_1 U_hubot"}, wantStatus: plan.StatusDone},
	{name: "add the viewer", step: step(domain.OpAddAssignees, "", "octocat"), reader: unassigned, wantCalls: []string{"AddAssignees I_1 U_1"}, wantStatus: plan.StatusDone},
	{name: "add already assigned", step: step(domain.OpAddAssignees, "", "octocat"), wantStatus: plan.StatusSkipped},
	{name: "add unknown login", step: step(domain.OpAddAssignees, "", "ghost,octocat"), wantErr: domain.ErrNotFound},
	{name: "add without resolver", step: step(domain.OpAddAssignees, "", "hubot,octocat"), bare: true, wantErr: domain.ErrNotImplemented},
	{name: "add assignee fails", step: step(domain.OpAddAssignees, "", "hubot,octocat"), writer: func(w *fakeWriter) { w.fail["AddAssignees"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	{name: "add assignee empty target", step: withTarget(step(domain.OpAddAssignees, "", "hubot"), ""), wantErr: domain.ErrUsage},
	{name: "remove assignee", step: step(domain.OpRemoveAssignees, "", ""), wantCalls: []string{"RemoveAssignees I_1 U_1"}, wantStatus: plan.StatusDone},
	{name: "remove nothing", step: step(domain.OpRemoveAssignees, "", "octocat"), wantStatus: plan.StatusSkipped},
	{name: "remove assignee empty target", step: withTarget(step(domain.OpRemoveAssignees, "", ""), ""), wantErr: domain.ErrUsage},
	{name: "add label", step: step(domain.OpAddLabels, "", "bug,feature"), wantCalls: []string{"AddLabels I_1 L_feature"}, wantStatus: plan.StatusDone},
	{name: "add label unknown", step: step(domain.OpAddLabels, "", "bug,urgent"), wantErr: domain.ErrNotFound},
	{name: "add label already present", step: step(domain.OpAddLabels, "", "bug"), wantStatus: plan.StatusSkipped},
	{name: "add label read fails", step: step(domain.OpAddLabels, "", "feature"), reader: func(r *fakeReader) { r.fail["RepositoryLabels"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	{name: "add label empty target", step: withTarget(step(domain.OpAddLabels, "", "feature"), ""), wantErr: domain.ErrUsage},
	{name: "remove label", step: step(domain.OpRemoveLabels, "", ""), wantCalls: []string{"RemoveLabels I_1 L_bug"}, wantStatus: plan.StatusDone},
	{name: "remove nothing", step: step(domain.OpRemoveLabels, "", "bug"), wantStatus: plan.StatusSkipped},
	{name: "remove label empty target", step: withTarget(step(domain.OpRemoveLabels, "", ""), ""), wantErr: domain.ErrUsage},
}

func TestExecuteSetOperations(t *testing.T) {
	runExecCases(t, setCases)
}
