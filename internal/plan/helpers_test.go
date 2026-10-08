package plan_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var (
	now     = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	viewer  = domain.Viewer{Login: "octocat", ID: "U_1", Host: "github.com", TokenSource: "gh"}
	project = domain.Project{
		Ref: domain.ProjectRef{Owner: "acme", Number: 7}, NodeID: "PVT_1", Title: "Board",
		Fields: []domain.Field{
			{ID: "F_status", Name: "Status", DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{{ID: "O_todo", Name: "Todo"}, {ID: "O_done", Name: "Done"}}},
			{ID: "F_sprint", Name: "Sprint", DataType: domain.DataTypeIteration, Iterations: []domain.Iteration{{ID: "IT_1", Title: "Sprint 1", Start: now, Duration: 14}}},
			{ID: "F_est", Name: "Estimate", DataType: domain.DataTypeNumber},
			{ID: "F_start", Name: "Start", DataType: domain.DataTypeDate},
			{ID: "F_notes", Name: "Notes", DataType: domain.DataTypeText},
			{ID: "F_title", Name: "Title", DataType: domain.DataTypeTitle},
		},
		Repositories: []domain.Repository{{NodeID: "R_1", Owner: "acme", Name: "repo"}},
	}
)

// issueItem is the open issue acme/repo#12 on the board, fresh per call.
func issueItem() domain.Item {
	return domain.Item{
		ProjectItemID: "PVTI_1",
		Issue: domain.Issue{
			NodeID: "I_1", Owner: "acme", Repo: "repo", Number: 12, Title: "Fix login", State: domain.IssueOpen,
			Labels:    []domain.Label{{ID: "L_bug", Name: "bug"}},
			Assignees: []domain.User{{ID: "U_1", Login: "octocat"}},
			Type:      &domain.IssueType{ID: "IT_task", Name: "Task"},
		},
		Values:      map[string]domain.FieldValue{"Status": {Field: "Status", Value: "Todo", OptionID: "O_todo"}},
		IssueFields: []domain.IssueFieldValue{{FieldID: "IF_start", Name: "Start date", Value: "2026-10-01"}},
	}
}

// epicItem is the epic acme/repo#5, a parent candidate.
func epicItem() domain.Item {
	return domain.Item{ProjectItemID: "PVTI_5", Issue: domain.Issue{NodeID: "I_epic", Owner: "acme", Repo: "repo", Number: 5, Title: "Epic", State: domain.IssueOpen}}
}

// target names issueItem for a step.
func target() domain.Target {
	return domain.Target{NodeID: "I_1", ProjectItemID: "PVTI_1", Repository: "acme/repo", Number: 12, Title: "Fix login"}
}

// step builds a step on issueItem with its Before read from the fixture.
func step(op domain.Operation, field, after string) domain.Step {
	s := domain.Step{Operation: op, Target: target(), Field: field, After: after, Description: "Passo de teste."}
	s.Before, _ = plan.Precondition(s, issueItem())
	return s
}

// closeStep is the one-step close plan every apply test starts from.
func closeStep() domain.Step {
	return step(domain.OpCloseIssue, "", string(domain.IssueClosed))
}

// newPlan seals a plan of the viewer over the fixture project.
func newPlan(t *testing.T, steps ...domain.Step) domain.Plan {
	t.Helper()
	p := domain.Plan{
		ID: "20261008T120000Z-abcd1234", KitVersion: "test", CreatedAt: now, Actor: viewer.Login, Host: viewer.Host,
		Project: project.Ref, ProjectID: project.NodeID, Command: "close acme/repo#12", Description: "Fechar a issue.", Steps: steps,
	}
	sealed, err := p.Seal()
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	return sealed
}

// newReader returns a reader over the fixtures with every optional port.
func newReader() *fakeReader {
	it := issueItem()
	return &fakeReader{
		project: project, viewer: viewer,
		items:      map[string]domain.Item{"I_1": it, "acme/repo#12": it, "I_epic": epicItem(), "acme/repo#5": epicItem()},
		labels:     []domain.Label{{ID: "L_bug", Name: "bug"}, {ID: "L_feature", Name: "feature"}},
		milestones: []domain.Milestone{{ID: "M_1", Number: 1, Title: "v1"}},
		types:      []domain.IssueType{{ID: "IT_task", Name: "Task"}, {ID: "IT_feature", Name: "Feature"}},
		comments:   map[string][]domain.Comment{},
		users:      map[string]string{"hubot": "U_hubot"},
		fields:     map[string]string{"Target date": "IF_target"},
		fail:       map[string]error{},
	}
}

// newWriter returns a writer whose creations carry fixed ids.
func newWriter() *fakeWriter {
	return &fakeWriter{
		fail: map[string]error{}, itemID: "PVTI_new", updateID: "PVTSU_1",
		issue:     domain.Issue{NodeID: "I_new", Owner: "acme", Repo: "repo", Number: 99, Title: "Nova tarefa"},
		comment:   domain.Comment{ID: "IC_1"},
		field:     domain.Field{ID: "F_new", Name: "Lane"},
		label:     domain.Label{ID: "L_new", Name: "lane"},
		milestone: domain.Milestone{ID: "M_new", Title: "v2"},
		project:   domain.Project{NodeID: "PVT_new", Ref: domain.ProjectRef{Owner: "acme", Number: 8}},
	}
}

func newPorts(r domain.ProjectReader, w domain.ProjectWriter) plan.Ports {
	return plan.Ports{Reader: r, Writer: w, Clock: fakeClock{now}}
}

// applyOptions returns interactive options over a temporary state dir.
func applyOptions(t *testing.T) (plan.ApplyOptions, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	return plan.ApplyOptions{Interactive: true, Viewer: viewer, StateDir: filepath.Join(t.TempDir(), "state"), Out: out}, out
}

// journalOf reads the journal of a plan.
func journalOf(t *testing.T, dir, id string) []plan.JournalEntry {
	t.Helper()
	entries, err := plan.NewJournal(dir).Read(id)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	return entries
}

// auditOf reads the audit log of a state directory.
func auditOf(t *testing.T, dir string) []audit.Entry {
	t.Helper()
	entries, err := audit.Query(dir, time.Time{})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return entries
}

// payload encodes a creation input the way producers do.
func payload(t *testing.T, in any) string {
	t.Helper()
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeFile writes a fixture file with owner-only permissions.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // fixture under t.TempDir()
		t.Fatal(err)
	}
}

// equalCalls compares recorded calls with the expected ones.
func equalCalls(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}
