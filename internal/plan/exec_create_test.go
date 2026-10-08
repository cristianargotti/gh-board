package plan_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

// creation builds a creation step with its payload.
func creation(t *testing.T, op domain.Operation, in any) domain.Step {
	t.Helper()
	return domain.Step{Operation: op, Target: domain.Target{Repository: "acme/repo"}, After: payload(t, in)}
}

func createCases(t *testing.T) []execCase {
	t.Helper()
	planID := newPlan(t, closeStep()).ID
	posted := func(r *fakeReader) {
		r.comments["I_1"] = []domain.Comment{{ID: "IC_old", Body: "x " + plan.Marker(planID, 0)}}
	}
	thisWeek := func(r *fakeReader) {
		r.updates = []plan.StatusUpdate{{ID: "PVTSU_old", CreatedAt: now.Add(-24 * time.Hour)}}
	}
	marked := func(r *fakeReader) {
		r.updates = []plan.StatusUpdate{{ID: "PVTSU_old", Body: plan.WeekMarker(now), CreatedAt: now.AddDate(0, 0, -30)}}
	}
	lastWeek := func(r *fakeReader) {
		r.updates = []plan.StatusUpdate{{ID: "PVTSU_old", CreatedAt: now.AddDate(0, 0, -7)}}
	}
	update := domain.StatusUpdateInput{ProjectID: "PVT_1", Body: "Resumo", Status: "ON_TRACK"}
	return []execCase{
		{name: "comment", step: step(domain.OpAddComment, "", "Feito."), wantCalls: []string{"AddComment I_1 Feito.\n\n" + plan.Marker(planID, 0)}, wantStatus: plan.StatusDone, wantNode: "IC_1"},
		{name: "comment already posted", step: step(domain.OpAddComment, "", "Feito."), reader: posted, wantStatus: plan.StatusSkipped, wantNode: "IC_old"},
		{name: "comment read fails", step: step(domain.OpAddComment, "", "Feito."), reader: func(r *fakeReader) { r.fail["IssueComments"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "comment write fails", step: step(domain.OpAddComment, "", "Feito."), writer: func(w *fakeWriter) { w.fail["AddComment"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "comment empty target", step: withTarget(step(domain.OpAddComment, "", "x"), ""), wantErr: domain.ErrUsage},
		{name: "create issue", step: creation(t, domain.OpCreateIssue, domain.CreateIssueInput{RepositoryID: "R_1", Title: "Nova"}), wantCalls: []string{"CreateIssue R_1 Nova"}, wantStatus: plan.StatusDone, wantNode: "I_new"},
		{name: "create issue bad payload", step: domain.Step{Operation: domain.OpCreateIssue, After: "nope"}, wantErr: domain.ErrUsage},
		{name: "create issue fails", step: creation(t, domain.OpCreateIssue, domain.CreateIssueInput{}), writer: func(w *fakeWriter) { w.fail["CreateIssue"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "status update", step: creation(t, domain.OpCreateStatusUpdate, update), reader: lastWeek, wantCalls: []string{"CreateStatusUpdate PVT_1 ON_TRACK"}, wantStatus: plan.StatusDone, wantNode: "PVTSU_1"},
		{name: "status update same week", step: creation(t, domain.OpCreateStatusUpdate, update), reader: thisWeek, wantStatus: plan.StatusSkipped, wantNode: "PVTSU_old"},
		{name: "status update marked", step: creation(t, domain.OpCreateStatusUpdate, update), reader: marked, wantStatus: plan.StatusSkipped, wantNode: "PVTSU_old"},
		{name: "status update without lister", step: creation(t, domain.OpCreateStatusUpdate, domain.StatusUpdateInput{Status: "AT_RISK"}), bare: true, wantErr: domain.ErrAPI},
		{name: "status update list fails", step: creation(t, domain.OpCreateStatusUpdate, update), reader: func(r *fakeReader) { r.fail["ListStatusUpdates"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "status update bad payload", step: domain.Step{Operation: domain.OpCreateStatusUpdate, After: "["}, wantErr: domain.ErrUsage},
		{name: "status update write fails", step: creation(t, domain.OpCreateStatusUpdate, update), writer: func(w *fakeWriter) { w.fail["CreateStatusUpdate"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
	}
}

func TestExecuteCommentsAndStatusUpdates(t *testing.T) {
	runExecCases(t, createCases(t))
}

func structureCases(t *testing.T) []execCase {
	t.Helper()
	field := domain.CreateFieldInput{ProjectID: "PVT_1", Name: "Lane", DataType: domain.DataTypeSingleSelect}
	return []execCase{
		{name: "create field", step: creation(t, domain.OpCreateField, field), wantCalls: []string{"CreateProjectField PVT_1 Lane SINGLE_SELECT"}, wantStatus: plan.StatusDone, wantNode: "F_new"},
		{name: "create field exists", step: creation(t, domain.OpCreateField, domain.CreateFieldInput{ProjectID: "PVT_1", Name: "status"}), wantStatus: plan.StatusSkipped, wantNode: "F_status"},
		{name: "create field on created project", step: creation(t, domain.OpCreateField, domain.CreateFieldInput{ProjectID: plan.Ref(0), Name: "Status"}), created: map[int]string{0: "PVT_new"}, wantCalls: []string{"CreateProjectField PVT_new Status"}, wantStatus: plan.StatusDone, wantNode: "F_new"},
		{name: "create field dangling reference", step: creation(t, domain.OpCreateField, domain.CreateFieldInput{ProjectID: plan.Ref(0)}), wantErr: domain.ErrUsage},
		{name: "create field bad payload", step: domain.Step{Operation: domain.OpCreateField, After: "x"}, wantErr: domain.ErrUsage},
		{name: "create field fails", step: creation(t, domain.OpCreateField, field), writer: func(w *fakeWriter) { w.fail["CreateProjectField"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "create label", step: creation(t, domain.OpCreateLabel, domain.CreateLabelInput{RepositoryID: "R_1", Name: "lane"}), wantCalls: []string{"CreateLabel R_1 lane"}, wantStatus: plan.StatusDone, wantNode: "L_new"},
		{name: "create label exists", step: creation(t, domain.OpCreateLabel, domain.CreateLabelInput{Name: "Bug"}), wantStatus: plan.StatusSkipped, wantNode: "L_bug"},
		{name: "create label read fails", step: creation(t, domain.OpCreateLabel, domain.CreateLabelInput{Name: "lane"}), reader: func(r *fakeReader) { r.fail["RepositoryLabels"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "create label bad payload", step: domain.Step{Operation: domain.OpCreateLabel, After: ""}, wantErr: domain.ErrUsage},
		{name: "create label fails", step: creation(t, domain.OpCreateLabel, domain.CreateLabelInput{Name: "lane"}), writer: func(w *fakeWriter) { w.fail["CreateLabel"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "create milestone", step: creation(t, domain.OpCreateMilestone, domain.CreateMilestoneInput{Owner: "acme", Repo: "repo", Title: "v2"}), wantCalls: []string{"CreateMilestone acme/repo v2"}, wantStatus: plan.StatusDone, wantNode: "M_new"},
		{name: "create milestone exists", step: creation(t, domain.OpCreateMilestone, domain.CreateMilestoneInput{Owner: "acme", Repo: "repo", Title: "V1"}), wantStatus: plan.StatusSkipped, wantNode: "M_1"},
		{name: "create milestone read fails", step: creation(t, domain.OpCreateMilestone, domain.CreateMilestoneInput{Title: "v2"}), reader: func(r *fakeReader) { r.fail["RepositoryMilestones"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "create milestone bad payload", step: domain.Step{Operation: domain.OpCreateMilestone, After: "1"}, wantErr: domain.ErrUsage},
		{name: "create milestone fails", step: creation(t, domain.OpCreateMilestone, domain.CreateMilestoneInput{Title: "v2"}), writer: func(w *fakeWriter) { w.fail["CreateMilestone"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "copy project", step: creation(t, domain.OpCopyProject, domain.CopyProjectInput{SourceProjectID: "PVT_tpl", Title: "Novo"}), wantCalls: []string{"CopyProject PVT_tpl Novo"}, wantStatus: plan.StatusDone, wantNode: "PVT_new"},
		{name: "copy project bad payload", step: domain.Step{Operation: domain.OpCopyProject, After: "{"}, wantErr: domain.ErrUsage},
		{name: "copy project fails", step: creation(t, domain.OpCopyProject, domain.CopyProjectInput{}), writer: func(w *fakeWriter) { w.fail["CopyProject"] = domain.ErrAPI }, wantErr: domain.ErrAPI},
		{name: "link repository", step: domain.Step{Operation: domain.OpLinkRepository, Target: domain.Target{NodeID: "PVT_1", Repository: "acme/other"}, After: "R_2"}, wantCalls: []string{"LinkProjectToRepository PVT_1 R_2"}, wantStatus: plan.StatusDone},
		{name: "link repository already linked", step: domain.Step{Operation: domain.OpLinkRepository, Target: domain.Target{NodeID: "PVT_1"}, After: "R_1"}, wantStatus: plan.StatusSkipped},
		{name: "link created project", step: domain.Step{Operation: domain.OpLinkRepository, Target: domain.Target{NodeID: plan.Ref(0)}, After: "R_1"}, created: map[int]string{0: "PVT_new"}, wantCalls: []string{"LinkProjectToRepository PVT_new R_1"}, wantStatus: plan.StatusDone},
		{name: "link dangling repository reference", step: domain.Step{Operation: domain.OpLinkRepository, Target: domain.Target{NodeID: "PVT_1"}, After: plan.Ref(9)}, wantErr: domain.ErrUsage},
		{name: "link empty target", step: domain.Step{Operation: domain.OpLinkRepository, After: "R_1"}, wantErr: domain.ErrUsage},
	}
}

func TestExecuteStructureOperations(t *testing.T) {
	runExecCases(t, structureCases(t))
}
