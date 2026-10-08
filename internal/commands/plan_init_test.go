package commands

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestPlanInitDiscovery(t *testing.T) {
	for _, from := range []string{"", "model/4"} {
		t.Run("from-"+from, func(t *testing.T) {
			planTestDiscoveryCase(t, from)
		})
	}
}

func planTestDiscoveryCase(t *testing.T, from string) {
	t.Helper()
	deps, reader, writer, _ := planTestDeps(t)
	reader.templateLabels = []domain.CreateLabelInput{{Name: "entrada", Color: "00ff00"}, {Name: "exists"}}
	reader.labels = []domain.Label{{Name: "exists"}}
	reader.templateMilestones = []domain.CreateMilestoneInput{{Title: "Release 3"}, {Title: "Release 1"}}
	output := filepath.Join(t.TempDir(), "board.yml")
	args := []string{"init", "--forms", "--output", output}
	if from != "" {
		args = append(args, "--from", from, "--title", "Equipe", "--owner", "tester")
	}
	err := planTestExecute(context.Background(), args, deps)
	if domain.CodeOf(err) != domain.ExitPlanRequired {
		t.Fatalf("init: %v", err)
	}
	p := planTestSaved(t, deps)
	if len(writer.calls) > 0 {
		t.Fatal("init performed remote writes")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("init wrote draft before apply")
	}
	if from != "" {
		planTestInitCopy(t, p)
	}
	var local planInitLocal
	if err := json.Unmarshal([]byte(p.Steps[len(p.Steps)-1].Field), &local); err != nil {
		t.Fatal(err)
	}
	if !local.Forms || local.Output != output || local.Config.Repository != "team/work" {
		t.Fatalf("local = %+v", local)
	}
}

func planTestInitCopy(t *testing.T, p domain.Plan) {
	t.Helper()
	want := []domain.Operation{domain.OpCopyProject, domain.OpCreateLabel, domain.OpCreateMilestone, domain.OpLinkRepository}
	if len(p.Steps) != len(want) {
		t.Fatalf("steps: %+v", p.Steps)
	}
	for i, operation := range want {
		if p.Steps[i].Operation != operation {
			t.Fatalf("step %d = %+v", i, p.Steps[i])
		}
	}
	var input domain.CopyProjectInput
	if err := json.Unmarshal([]byte(p.Steps[0].After), &input); err != nil {
		t.Fatal(err)
	}
	if input.OwnerID != "U_TEST" || input.IncludeDraftIssues || input.Title != "Equipe" {
		t.Fatalf("copy: %+v", input)
	}
	if p.Steps[3].Target.NodeID != "step:0" {
		t.Fatal("repository is not linked to the new project")
	}
}

var planInitFailures = []struct {
	name   string
	change func(*Deps, *planTestReader)
	code   domain.ExitCode
}{
	{"issue types", func(_ *Deps, r *planTestReader) { r.fail["types"] = errors.New("offline") }, domain.ExitAPI},
	{"discover", func(_ *Deps, r *planTestReader) { r.fail["discover"] = errors.New("offline") }, domain.ExitAPI},
	{"template", func(_ *Deps, r *planTestReader) { r.fail["template"] = errors.New("offline") }, domain.ExitAPI},
	{"missing config", func(_ *Deps, r *planTestReader) { r.template = nil }, domain.ExitPlanRequired},
	{"missing reader", func(d *Deps, r *planTestReader) { d.Reader = struct{ domain.ProjectReader }{r} }, domain.ExitAPI},
	{"owner", func(_ *Deps, r *planTestReader) { r.fail["owner"] = errors.New("offline") }, domain.ExitAPI},
	{"owner id", func(_ *Deps, r *planTestReader) { r.ownerID = "" }, domain.ExitNotFound},
	{"repository", func(_ *Deps, r *planTestReader) { r.fail["repository"] = errors.New("offline") }, domain.ExitAPI},
	{"repository id", func(_ *Deps, r *planTestReader) { r.repoID = "" }, domain.ExitNotFound},
	{"labels", func(_ *Deps, r *planTestReader) { r.fail["labels"] = errors.New("offline") }, domain.ExitAPI},
	{"milestones", func(_ *Deps, r *planTestReader) { r.fail["milestones"] = errors.New("offline") }, domain.ExitAPI},
	{"blank label", func(_ *Deps, r *planTestReader) { r.templateLabels = []domain.CreateLabelInput{{Name: " "}} }, domain.ExitUsage},
	{"blank milestone", func(_ *Deps, r *planTestReader) { r.templateMilestones = []domain.CreateMilestoneInput{{Title: " "}} }, domain.ExitUsage},
}

func TestPlanInitFailures(t *testing.T) {
	for _, tc := range planInitFailures {
		t.Run(tc.name, func(t *testing.T) {
			deps, reader, writer, _ := planTestDeps(t)
			tc.change(deps, reader)
			err := planTestExecute(context.Background(), []string{"init", "--from", "model/4", "--output", filepath.Join(t.TempDir(), "board.yml")}, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("error = %v", err)
			}
			if len(writer.calls) > 0 {
				t.Fatal("unexpected mutation")
			}
		})
	}
}

func TestPlanInitExistingOutput(t *testing.T) {
	for _, name := range []string{"board.yml", ".github/ISSUE_TEMPLATE/tarefa.yml"} {
		t.Run(name, func(t *testing.T) {
			deps, _, _, _ := planTestDeps(t)
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := planTestExecute(context.Background(), []string{"init", "--forms", "--output", filepath.Join(dir, "board.yml")}, deps)
			if domain.CodeOf(err) != domain.ExitUsage {
				t.Fatalf("error = %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "keep" {
				t.Fatal("existing file changed")
			}
		})
	}
}

func TestPlanTemplateDeduplication(t *testing.T) {
	for _, kind := range []string{"label", "milestone"} {
		t.Run(kind, func(t *testing.T) {
			var steps []domain.Step
			var err error
			if kind == "label" {
				steps, err = planTemplateLabels("team/work", "R_TEST", []domain.CreateLabelInput{{Name: "one"}, {Name: "one"}}, nil)
			} else {
				steps, err = planTemplateMilestones("team", "work", []domain.CreateMilestoneInput{{Title: "one"}, {Title: "one"}}, nil)
			}
			if err != nil || len(steps) != 1 {
				t.Fatalf("steps %v, error %v", steps, err)
			}
		})
	}
}

func TestPlanInitCopyRecovery(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "copy", true: "resume"}[journal], func(t *testing.T) {
			planTestRecoveryCase(t, journal)
		})
	}
}

func planTestRecoveryCase(t *testing.T, journal bool) {
	t.Helper()
	deps, reader, writer, _ := planTestDeps(t)
	writer.project.NodeID = "PVT_NEW"
	p := domain.Plan{ID: "init-test", Command: "init", Steps: []domain.Step{{Operation: domain.OpCopyProject}}}
	wrapped := &planApplyWriter{ProjectWriter: writer, reader: reader, state: deps.Dirs.State, p: p}
	if journal {
		err := plan.NewJournal(deps.Dirs.State).Append(plan.JournalEntry{PlanID: p.ID, Operation: domain.OpCopyProject, Created: "PVT_NEW", Status: plan.StatusDone})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err := wrapped.CopyProject(context.Background(), domain.CopyProjectInput{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := wrapped.LinkProjectToRepository(context.Background(), "PVT_SOURCE", "R_TEST"); err != nil {
		t.Fatal(err)
	}
	if writer.linked != "PVT_NEW" {
		t.Fatal("linked the source template")
	}
	if _, err := wrapped.planInitProject(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPlanEmbeddedTemplate(t *testing.T) {
	reference, err := planReferenceConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg, labels, milestones, err := planEmbeddedTemplate(reference.Project, time.UTC)
	if err != nil || cfg == nil || len(labels) == 0 || len(milestones) != 0 {
		t.Fatalf("embedded template: %v, %+v, %d labels", err, cfg, len(labels))
	}
	if !domain.Contains(cfg.Template.LabelNames(), reference.Capabilities.Triage.Label) {
		t.Fatal("the triage label is a template label")
	}
	other, labels, _, err := planEmbeddedTemplate(domain.ProjectRef{Owner: "model", Number: 4}, time.UTC)
	if err != nil || other != nil || labels != nil {
		t.Fatalf("another project has no embedded template: %v, %+v", err, other)
	}
}
