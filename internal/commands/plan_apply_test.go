package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var planApplyCases = []struct {
	name             string
	dry, interactive bool
	code             domain.ExitCode
	writes           int
}{
	{"apply", false, true, domain.ExitOK, 1},
	{"dry", true, false, domain.ExitOK, 0},
	{"terminal", false, false, domain.ExitApplyRefused, 0},
	{"drift", false, true, domain.ExitDrift, 0},
	{"failure", false, true, domain.ExitAPI, 1},
	{"actor", false, true, domain.ExitApplyRefused, 0},
}

func TestPlanApply(t *testing.T) {
	for index, tc := range planApplyCases {
		t.Run(tc.name, func(t *testing.T) {
			planTestApplyCase(t, index)
		})
	}
}

func planTestApplyCase(t *testing.T, index int) {
	t.Helper()
	tc := planApplyCases[index]
	deps, reader, writer, _ := planTestDeps(t)
	if err := planTestExecute(context.Background(), []string{"close", "#7"}, deps); domain.CodeOf(err) != domain.ExitPlanRequired {
		t.Fatal(err)
	}
	p := planTestSaved(t, deps)
	deps.Flags.DryRun = tc.dry
	if tc.name == "drift" {
		reader.items[0].Issue.State = "UNKNOWN"
	}
	if tc.name == "failure" {
		writer.failure = domain.ErrAPI
	}
	if tc.name == "actor" {
		reader.viewer.Login = "other"
	}
	err := planApplyRun(context.Background(), deps, p, reader.viewer, tc.interactive)
	if domain.CodeOf(err) != tc.code || len(writer.calls) != tc.writes {
		t.Fatalf("error %v, calls %v", err, writer.calls)
	}
	if tc.name == "apply" {
		if err := planApplyRun(context.Background(), deps, p, reader.viewer, true); err != nil {
			t.Fatal(err)
		}
		if len(writer.calls) != 1 {
			t.Fatal("repeated completed step")
		}
	}
}

func TestPlanApplyCommand(t *testing.T) {
	for _, ref := range []string{"id", "path", "missing"} {
		t.Run(ref, func(t *testing.T) {
			planTestApplyCommandCase(t, ref)
		})
	}
}

func planTestApplyCommandCase(t *testing.T, ref string) {
	t.Helper()
	deps, _, _, _ := planTestDeps(t)
	if err := planTestExecute(context.Background(), []string{"close", "#7"}, deps); domain.CodeOf(err) != domain.ExitPlanRequired {
		t.Fatal(err)
	}
	p := planTestSaved(t, deps)
	arg := p.ID
	if ref == "path" {
		arg = plan.Path(deps.Dirs.State, p.ID)
	}
	if ref == "missing" {
		arg = "missing-plan"
	}
	err := planTestExecute(context.Background(), []string{"apply", arg, "--dry-run", "--json"}, deps)
	want := domain.ExitOK
	if ref == "missing" {
		want = domain.ExitNotFound
	}
	if domain.CodeOf(err) != want {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanApplyInit(t *testing.T) {
	for _, copying := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "copied"}[copying], func(t *testing.T) {
			planTestApplyInitCase(t, copying)
		})
	}
}

func planTestApplyInitCase(t *testing.T, copying bool) {
	t.Helper()
	deps, reader, writer, _ := planTestDeps(t)
	writer.project.NodeID = "PVT_COPY"
	output := filepath.Join(t.TempDir(), "board.yml")
	args := []string{"init", "--forms", "--output", output}
	if copying {
		args = append(args, "--from", "model/4")
	}
	if err := planTestExecute(context.Background(), args, deps); domain.CodeOf(err) != domain.ExitPlanRequired {
		t.Fatal(err)
	}
	p := planTestSaved(t, deps)
	if err := planApplyRun(context.Background(), deps, p, reader.viewer, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
	if copying && writer.linked != "PVT_COPY" {
		t.Fatalf("linked = %s", writer.linked)
	}
	count := len(writer.calls)
	if err := planApplyRun(context.Background(), deps, p, reader.viewer, true); err != nil {
		t.Fatal("resume:", err)
	}
	if len(writer.calls) != count {
		t.Fatal("init replayed remote steps")
	}
}

var planApplyWriterFailures = []struct {
	name      string
	input     domain.StatusUpdateInput
	configure func(*planApplyWriter, *planTestReader)
	code      domain.ExitCode
}{
	{"no lister", domain.StatusUpdateInput{}, func(w *planApplyWriter, r *planTestReader) { w.reader = struct{ domain.ProjectReader }{r} }, domain.ExitAPI},
	{"no date", domain.StatusUpdateInput{}, func(_ *planApplyWriter, _ *planTestReader) {}, domain.ExitApplyRefused},
	{"no marker", domain.StatusUpdateInput{StartDate: &planTestNow}, func(_ *planApplyWriter, _ *planTestReader) {}, domain.ExitApplyRefused},
	{"history", domain.StatusUpdateInput{StartDate: &planTestNow, Body: plan.WeekMarker(planTestNow)}, func(_ *planApplyWriter, r *planTestReader) { r.fail["updates"] = errors.New("offline") }, domain.ExitAPI},
}

func TestPlanApplyWriterFailures(t *testing.T) {
	for _, tc := range planApplyWriterFailures {
		t.Run(tc.name, func(t *testing.T) {
			_, reader, writer, _ := planTestDeps(t)
			wrapped := &planApplyWriter{ProjectWriter: writer, reader: reader, state: t.TempDir()}
			tc.configure(wrapped, reader)
			_, err := wrapped.CreateStatusUpdate(context.Background(), tc.input)
			if domain.CodeOf(err) != tc.code {
				t.Fatal(err)
			}
			if len(writer.calls) > 0 {
				t.Fatal("write after refusal")
			}
		})
	}
}

func TestPlanApplyInitInvalidMetadata(t *testing.T) {
	for _, payload := range []string{"{", "{}"} {
		t.Run(payload, func(t *testing.T) {
			_, reader, writer, _ := planTestDeps(t)
			wrapped := &planApplyWriter{ProjectWriter: writer, reader: reader, p: domain.Plan{Command: "init", Project: reader.project.Ref, Steps: []domain.Step{{Operation: domain.OpLinkRepository, Field: payload}}}}
			if err := wrapped.planFinishInit(context.Background()); err == nil {
				t.Fatal("accepted invalid local metadata")
			}
		})
	}
}

func TestPlanApplyMissingCopiedProject(t *testing.T) {
	for _, kind := range []string{"no-copy", "missing-journal", "no-resolver"} {
		t.Run(kind, func(t *testing.T) {
			planTestMissingCopyCase(t, kind)
		})
	}
}

func planTestMissingCopyCase(t *testing.T, kind string) {
	t.Helper()
	deps, reader, writer, _ := planTestDeps(t)
	wrapped := &planApplyWriter{ProjectWriter: writer, reader: reader, state: deps.Dirs.State, p: domain.Plan{ID: "init-test", Command: "init"}}
	if kind != "no-copy" {
		wrapped.p.Steps = []domain.Step{{Operation: domain.OpCopyProject}}
	}
	if kind == "no-resolver" {
		wrapped.reader = struct{ domain.ProjectReader }{reader}
		if err := plan.NewJournal(deps.Dirs.State).Append(plan.JournalEntry{PlanID: "init-test", Operation: domain.OpCopyProject, Created: "NEW", Status: plan.StatusDone}); err != nil {
			t.Fatal(err)
		}
		_, err := wrapped.planInitProject(context.Background())
		if err == nil || !strings.Contains(err.Error(), "discovery") {
			t.Fatal(err)
		}
	} else if err := wrapped.LinkProjectToRepository(context.Background(), "", "R_TEST"); err == nil {
		t.Fatal("missing project accepted")
	}
}
