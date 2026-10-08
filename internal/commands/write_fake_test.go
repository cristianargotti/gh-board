package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

type writeFake struct {
	failureErr error
	domain.ProjectReader
	domain.ProjectWriter
	project   domain.Project
	items     map[string]domain.Item
	viewer    domain.Viewer
	perm      domain.Permission
	calls     []string
	reads     int
	inputs    []domain.ItemFieldValueInput
	created   domain.CreateIssueInput
	failure   string
	failAt    int
	changeAt  int
	change    func(*writeFake)
	labels    []domain.Label
	users     []domain.User
	fields    map[string]string
	listPages []domain.ItemPage
}

type writeClock struct{}

func (writeClock) Now() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

func writeFixture(t *testing.T) (*Deps, *writeFake, *bytes.Buffer) {
	t.Helper()
	fake := &writeFake{
		project: writeProject(), items: map[string]domain.Item{},
		viewer: domain.Viewer{Login: "alice", ID: "U_alice", Host: "github.example"}, perm: domain.PermissionWrite,
		labels: []domain.Label{{ID: "L_bug", Name: "bug"}, {ID: "L_triage", Name: "triage"}, {ID: "L_old", Name: "old"}},
		users:  []domain.User{{ID: "U_bob", Login: "bob"}}, fields: map[string]string{"Start": "IF_start", "Target": "IF_target"},
	}
	for i := 1; i <= 3; i++ {
		it := writeItem(i)
		fake.items[it.Issue.NodeID] = it
	}
	out := &bytes.Buffer{}
	deps := &Deps{
		Reader: fake, Writer: fake, Clock: writeClock{}, Config: &config.Loaded{Config: writeConfiguration()},
		Dirs: config.Dirs{State: t.TempDir()}, Out: out, Err: &bytes.Buffer{}, Version: "test",
	}
	return deps, fake, out
}

func writeConfiguration() *domain.Config {
	return &domain.Config{
		Version: 1, Project: domain.ProjectRef{Owner: "team", Number: 1}, Repository: "team/work",
		Capabilities: domain.Capabilities{
			Status: &domain.StatusCapability{Field: "Flow", Ready: []string{"Ready"}, Active: []string{"Active"}, Done: []string{"Done"}},
			Task:   &domain.TaskCapability{IssueType: "Task", MaxEstimateDays: 3}, Epic: &domain.EpicCapability{IssueType: "Feature", Field: "Epic"},
			Lane: &domain.FieldCapability{Field: "Lane"}, Sprint: &domain.FieldCapability{Field: "Cycle"}, Estimate: &domain.FieldCapability{Field: "Size"},
			Dates:  &domain.DatesCapability{Start: "Start", Target: "Target", Source: domain.DateSourceProject},
			Triage: &domain.TriageCapability{Label: "triage"},
		},
		Policy: domain.Policy{Transitions: map[string][]string{"Ready": {"Active", "Done"}}, WIP: map[string]int{"Active": 3}, BulkThreshold: 2},
	}
}

func writeProject() domain.Project {
	now := writeClock{}.Now()
	return domain.Project{
		Ref: domain.ProjectRef{Owner: "team", Number: 1}, NodeID: "PVT_board", ViewerRole: domain.RoleWriter,
		Repositories: []domain.Repository{{Owner: "team", Name: "work", NodeID: "R_work"}}, Fields: []domain.Field{
			{ID: "F_flow", Name: "Flow", DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{{ID: "O_ready", Name: "Ready"}, {ID: "O_active", Name: "Active"}, {ID: "O_done", Name: "Done"}}},
			{ID: "F_lane", Name: "Lane", DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{{ID: "O_platform", Name: "Platform"}}},
			{ID: "F_epic", Name: "Epic", DataType: domain.DataTypeText},
			{ID: "F_size", Name: "Size", DataType: domain.DataTypeNumber},
			{ID: "F_start", Name: "Start", DataType: domain.DataTypeDate},
			{ID: "F_target", Name: "Target", DataType: domain.DataTypeDate},
			{ID: "F_cycle", Name: "Cycle", DataType: domain.DataTypeIteration, Iterations: []domain.Iteration{
				{ID: "IT_current", Title: "Cycle 1", Start: now.AddDate(0, 0, -2), Duration: 7},
				{ID: "IT_next", Title: "Cycle 2", Start: now.AddDate(0, 0, 5), Duration: 7},
			}},
		},
	}
}

func writeItem(number int) domain.Item {
	return domain.Item{
		ProjectItemID: fmt.Sprintf("PVTI_%d", number),
		Issue: domain.Issue{
			NodeID: fmt.Sprintf("I_kwDOTest%04d", number), Owner: "team", Repo: "work", Number: number, Title: "Task",
			State: domain.IssueOpen, Type: &domain.IssueType{ID: "T_task", Name: "Task"},
			Assignees: []domain.User{{ID: "U_bob", Login: "bob"}}, Labels: []domain.Label{{ID: "L_old", Name: "old"}},
		},
		Values:      map[string]domain.FieldValue{"Flow": {Field: "Flow", Value: "Ready"}},
		IssueFields: []domain.IssueFieldValue{{FieldID: "IF_start", Name: "Start", Value: "2026-10-01"}},
	}
}

func (f *writeFake) writeErrorFor(name string) error {
	f.calls = append(f.calls, name)
	if f.failure == name {
		if f.failureErr != nil {
			return f.failureErr
		}
		return errors.New("fake API failure")
	}
	return nil
}

func (f *writeFake) DiscoverProject(context.Context, domain.ProjectRef) (domain.Project, error) {
	return f.project, f.writeErrorFor("discover")
}

func (f *writeFake) Viewer(context.Context) (domain.Viewer, error) {
	return f.viewer, f.writeErrorFor("viewer")
}

func (f *writeFake) ViewerPermission(context.Context, string, string) (domain.Permission, error) {
	return f.perm, f.writeErrorFor("permission")
}

func (f *writeFake) GetItem(_ context.Context, _ domain.Project, ref domain.Reference) (domain.Item, error) {
	f.reads++
	if err := f.writeErrorFor("read"); err != nil {
		return domain.Item{}, err
	}
	if f.reads == f.failAt {
		return domain.Item{}, domain.ErrNotFound
	}
	if f.reads == f.changeAt && f.change != nil {
		f.change(f)
	}
	for _, it := range f.items {
		if ref.NodeID == it.Issue.NodeID || ref.NodeID == it.ProjectItemID || (ref.Number == it.Issue.Number && ref.Owner == it.Issue.Owner && ref.Repo == it.Issue.Repo) {
			return it, nil
		}
	}
	return domain.Item{}, domain.ErrNotFound
}

func (f *writeFake) ListItems(_ context.Context, _ domain.Project, opts domain.ListOptions) (domain.ItemPage, error) {
	if len(f.listPages) > 0 {
		index := 0
		if opts.Cursor != "" && len(f.listPages) > 1 {
			index = 1
		}
		return f.listPages[index], f.writeErrorFor("list")
	}
	page := domain.ItemPage{}
	for _, it := range f.items {
		page.Items = append(page.Items, it)
	}
	return page, f.writeErrorFor("list")
}

func (f *writeFake) RepositoryLabels(context.Context, string, string) ([]domain.Label, error) {
	return f.labels, f.writeErrorFor("labels")
}

func (f *writeFake) RepositoryMilestones(context.Context, string, string) ([]domain.Milestone, error) {
	return []domain.Milestone{{ID: "M_one", Title: "Release"}}, f.writeErrorFor("milestones")
}

func (f *writeFake) IssueTypes(context.Context, string) ([]domain.IssueType, error) {
	return []domain.IssueType{{ID: "T_task", Name: "Task"}, {ID: "T_epic", Name: "Feature"}}, f.writeErrorFor("types")
}

func (f *writeFake) AssignableUsers(context.Context, string, string) ([]domain.User, error) {
	return f.users, f.writeErrorFor("users")
}

func (f *writeFake) UserID(_ context.Context, login string) (string, error) {
	if err := f.writeErrorFor("userID"); err != nil {
		return "", err
	}
	if login == "carol" {
		return "U_carol", nil
	}
	return "", domain.ErrNotFound
}

func (f *writeFake) IssueFieldID(_ context.Context, _, name string) (string, error) {
	return f.fields[name], f.writeErrorFor("issueFieldID")
}

func (f *writeFake) RepositoryID(context.Context, string, string) (string, error) {
	return "R_work", f.writeErrorFor("repositoryID")
}

func (f *writeFake) IssueByID(_ context.Context, id string) (domain.Issue, error) {
	it, err := f.GetItem(context.Background(), f.project, domain.Reference{Kind: domain.RefNodeID, NodeID: id})
	return it.Issue, err
}

func writeTestRoot(deps *Deps) *cobra.Command {
	return NewRoot(deps)
}

func writeTestExecute(ctx context.Context, args []string, deps *Deps) error {
	root := writeTestRoot(deps)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
