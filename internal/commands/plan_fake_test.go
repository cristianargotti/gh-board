package commands

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var planTestNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

type planTestClock struct{}

func (planTestClock) Now() time.Time { return planTestNow }

type planTestReader struct {
	domain.ProjectReader
	project            domain.Project
	viewer             domain.Viewer
	permission         domain.Permission
	items              []domain.Item
	fresh              *domain.Item
	reads              int
	pages              map[string]domain.ItemPage
	milestones         []domain.Milestone
	labels             []domain.Label
	updates            []plan.StatusUpdate
	template           *domain.Config
	templateLabels     []domain.CreateLabelInput
	templateMilestones []domain.CreateMilestoneInput
	fail               map[string]error
	ownerID, repoID    string
}

func (r *planTestReader) DiscoverProject(_ context.Context, ref domain.ProjectRef) (domain.Project, error) {
	p := r.project
	p.Ref = ref
	return p, r.fail["discover"]
}

func (r *planTestReader) Viewer(context.Context) (domain.Viewer, error) {
	return r.viewer, r.fail["viewer"]
}

func (r *planTestReader) ViewerPermission(context.Context, string, string) (domain.Permission, error) {
	return r.permission, r.fail["permission"]
}

func (r *planTestReader) GetItem(_ context.Context, _ domain.Project, ref domain.Reference) (domain.Item, error) {
	r.reads++
	if r.fail["item"] != nil {
		return domain.Item{}, r.fail["item"]
	}
	if r.reads > 1 && r.fresh != nil {
		return *r.fresh, nil
	}
	for _, item := range r.items {
		if item.Issue.NodeID == ref.NodeID || item.Issue.Number == ref.Number {
			return item, nil
		}
	}
	return domain.Item{}, domain.ErrNotFound
}

func (r *planTestReader) ListItems(_ context.Context, _ domain.Project, opts domain.ListOptions) (domain.ItemPage, error) {
	if r.pages != nil {
		return r.pages[opts.Cursor], r.fail["list"]
	}
	return domain.ItemPage{Items: r.items}, r.fail["list"]
}

func (r *planTestReader) RepositoryLabels(context.Context, string, string) ([]domain.Label, error) {
	return r.labels, r.fail["labels"]
}

func (r *planTestReader) RepositoryMilestones(context.Context, string, string) ([]domain.Milestone, error) {
	return r.milestones, r.fail["milestones"]
}

func (r *planTestReader) ListStatusUpdates(context.Context, string) ([]plan.StatusUpdate, error) {
	return r.updates, r.fail["updates"]
}

func (r *planTestReader) TemplateConfig(context.Context, domain.ProjectRef) (*domain.Config, []domain.CreateLabelInput, []domain.CreateMilestoneInput, error) {
	return r.template, r.templateLabels, r.templateMilestones, r.fail["template"]
}

func (r *planTestReader) OwnerID(context.Context, string) (string, error) {
	return r.ownerID, r.fail["owner"]
}

func (r *planTestReader) RepositoryID(context.Context, string, string) (string, error) {
	return r.repoID, r.fail["repository"]
}

func (r *planTestReader) ProjectByID(context.Context, string) (domain.Project, error) {
	return r.project, r.fail["project-id"]
}

func (r *planTestReader) IssueTimeline(context.Context, string) (domain.IssueTimeline, error) {
	return domain.IssueTimeline{Complete: true}, r.fail["timeline"]
}

func (r *planTestReader) IssueComments(context.Context, string) ([]domain.Comment, error) {
	return nil, r.fail["comments"]
}

type planTestWriter struct {
	domain.ProjectWriter
	calls   []string
	update  domain.UpdateIssueInput
	status  domain.StatusUpdateInput
	copy    domain.CopyProjectInput
	project domain.Project
	linked  string
	failure error
}

func (w *planTestWriter) UpdateIssue(_ context.Context, in domain.UpdateIssueInput) (domain.Issue, error) {
	w.calls = append(w.calls, "update")
	w.update = in
	return domain.Issue{NodeID: in.IssueID}, w.failure
}

func (w *planTestWriter) CloseIssue(context.Context, string) error {
	w.calls = append(w.calls, "close")
	return w.failure
}

func (w *planTestWriter) CopyProject(_ context.Context, in domain.CopyProjectInput) (domain.Project, error) {
	w.calls = append(w.calls, "copy")
	w.copy = in
	return w.project, w.failure
}

func (w *planTestWriter) LinkProjectToRepository(_ context.Context, project, _ string) error {
	w.calls = append(w.calls, "link")
	w.linked = project
	return w.failure
}

func (w *planTestWriter) CreateStatusUpdate(_ context.Context, in domain.StatusUpdateInput) (string, error) {
	w.calls = append(w.calls, "status")
	w.status = in
	return "UPDATE_1", w.failure
}

func (w *planTestWriter) CreateLabel(context.Context, domain.CreateLabelInput) (domain.Label, error) {
	w.calls = append(w.calls, "label")
	return domain.Label{ID: "L_1"}, w.failure
}

func (w *planTestWriter) CreateMilestone(context.Context, domain.CreateMilestoneInput) (domain.Milestone, error) {
	w.calls = append(w.calls, "milestone")
	return domain.Milestone{ID: "M_3"}, w.failure
}

func planTestDeps(t *testing.T) (*Deps, *planTestReader, *planTestWriter, *bytes.Buffer) {
	t.Helper()
	project := domain.Project{
		Ref: domain.ProjectRef{Owner: "team", Number: 8}, NodeID: "PVT_TEST", Title: "Board", ViewerRole: domain.RoleWriter,
		Fields:       []domain.Field{{ID: "F_FLOW", Name: "Flow", DataType: domain.DataTypeText}},
		Repositories: []domain.Repository{{NodeID: "R_TEST", Owner: "team", Name: "work"}},
	}
	cfg := &domain.Config{Version: 1, Project: project.Ref, Repository: "team/work", Timezone: "UTC"}
	r := &planTestReader{
		project: project, viewer: domain.Viewer{Login: "tester", ID: "U_TEST", Host: "github.test"}, permission: domain.PermissionWrite,
		items:      []domain.Item{{ProjectItemID: "PVTI_TEST", Issue: domain.Issue{NodeID: "I_TEST", Owner: "team", Repo: "work", Number: 7, Title: "Entrega\x1b[31m", State: domain.IssueOpen}, Values: map[string]domain.FieldValue{"Flow": {Value: "Doing"}}}},
		milestones: []domain.Milestone{{ID: "M_1", Title: "Release 1"}, {ID: "M_2", Title: "Release 2"}}, fail: make(map[string]error), template: cfg, ownerID: "O_TEST", repoID: "R_TEST",
	}
	w := &planTestWriter{project: project}
	out := &bytes.Buffer{}
	deps := &Deps{
		Reader: r, Writer: w, Clock: planTestClock{}, Config: &config.Loaded{Config: cfg, Source: config.SourceFlag},
		Dirs: config.Paths(t.TempDir()), Out: out, Err: &bytes.Buffer{}, Version: "test",
	}
	return deps, r, w, out
}

func planTestSaved(t *testing.T, deps *Deps) domain.Plan {
	t.Helper()
	plans, err := plan.List(deps.Dirs.State)
	if err != nil || len(plans) != 1 {
		t.Fatalf("saved plans = %d, %v", len(plans), err)
	}
	return plans[0]
}

func planTestExecute(ctx context.Context, args []string, deps *Deps) error {
	return Execute(ctx, args, deps)
}

func (r *planTestReader) IssueTypes(context.Context, string) ([]domain.IssueType, error) {
	return []domain.IssueType{{Name: "Feature"}, {Name: "Task"}}, r.fail["types"]
}
