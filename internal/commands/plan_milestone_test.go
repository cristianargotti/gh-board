package commands

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var planMilestoneCases = []struct {
	name   string
	args   []string
	code   domain.ExitCode
	writes int
}{
	{"set", []string{"milestone", "set", "#7", "Release 1", "--reason", "delivery", "--expect", "Flow=Doing"}, domain.ExitOK, 1},
	{"dry", []string{"milestone", "set", "#7", "Release 1", "--dry-run"}, domain.ExitOK, 0},
	{"suggest", []string{"milestone", "set", "#7", "Relese 1"}, domain.ExitNotFound, 0},
	{"expect", []string{"milestone", "set", "#7", "Release 1", "--expect", "milestone=Release 2"}, domain.ExitDrift, 0},
	{"create", []string{"milestone", "create", "Release 3", "--due", "2026-12-01", "--description", "Entrega"}, domain.ExitPlanRequired, 0},
	{"exists", []string{"milestone", "create", "Release 1"}, domain.ExitOK, 0},
	{"bad due", []string{"milestone", "create", "Release 3", "--due", "tomorrow"}, domain.ExitUsage, 0},
	{"empty", []string{"milestone", "create", " "}, domain.ExitUsage, 0},
	{"same", []string{"milestone", "retarget", "Release 1", "Release 1"}, domain.ExitOK, 0},
	{"retarget", []string{"milestone", "retarget", "Release 1", "Release 2"}, domain.ExitPlanRequired, 0},
}

func TestPlanMilestoneCommands(t *testing.T) {
	for index, tc := range planMilestoneCases {
		t.Run(tc.name, func(t *testing.T) {
			planTestMilestoneCase(t, index)
		})
	}
}

func planTestMilestoneCase(t *testing.T, index int) {
	t.Helper()
	tc := planMilestoneCases[index]
	deps, reader, writer, out := planTestDeps(t)
	if tc.name == "retarget" {
		reader.items[0].Issue.Milestone = &reader.milestones[0]
	}
	err := planTestExecute(context.Background(), tc.args, deps)
	if domain.CodeOf(err) != tc.code || len(writer.calls) != tc.writes {
		t.Fatalf("result %v, writes %v", err, writer.calls)
	}
	if tc.name == "suggest" && !strings.Contains(err.Error(), "did you mean") {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("unsanitized output")
	}
	if tc.writes == 1 {
		planTestMilestoneAudit(t, deps, writer)
	}
	if tc.code == domain.ExitPlanRequired {
		if err := planTestSaved(t, deps).Verify(); err != nil {
			t.Fatal(err)
		}
	}
}

func planTestMilestoneAudit(t *testing.T, deps *Deps, writer *planTestWriter) {
	t.Helper()
	if writer.update.MilestoneID == nil || *writer.update.MilestoneID != "M_1" || writer.update.Title != nil || writer.update.Body != nil {
		t.Fatalf("update = %+v", writer.update)
	}
	p := planTestSaved(t, deps)
	entries, err := plan.NewJournal(deps.Dirs.State).Read(p.ID)
	if err != nil || len(entries) != 1 || entries[0].Status != plan.StatusDone {
		t.Fatalf("journal %+v, %v", entries, err)
	}
	audits, err := audit.Query(deps.Dirs.State, time.Time{})
	if err != nil || len(audits) != 1 || audits[0].Reason != "delivery" {
		t.Fatalf("audit %+v, %v", audits, err)
	}
}

var planMilestoneFailureCases = []struct {
	name      string
	configure func(*Deps, *planTestReader, *planTestWriter)
	code      domain.ExitCode
}{
	{"read", func(_ *Deps, r *planTestReader, _ *planTestWriter) { r.fail["milestones"] = errors.New("offline") }, domain.ExitAPI},
	{"write", func(_ *Deps, _ *planTestReader, w *planTestWriter) { w.failure = errors.New("offline") }, domain.ExitAPI},
	{"permission", func(_ *Deps, r *planTestReader, _ *planTestWriter) { r.permission = domain.PermissionRead }, domain.ExitPolicy},
	{"project", func(_ *Deps, r *planTestReader, _ *planTestWriter) { r.project.ViewerRole = domain.RoleReader }, domain.ExitPolicy},
	{"concurrent", func(_ *Deps, r *planTestReader, _ *planTestWriter) {
		snapshot := r.items[0]
		snapshot.Issue.Milestone = &r.milestones[1]
		r.fresh = &snapshot
	}, domain.ExitDrift},
	{"ambiguous", func(_ *Deps, r *planTestReader, _ *planTestWriter) {
		r.milestones = append(r.milestones, r.milestones[0])
	}, domain.ExitNotFound},
	{"no id", func(_ *Deps, r *planTestReader, _ *planTestWriter) { r.milestones[0].ID = "" }, domain.ExitAPI},
	{"no writer", func(d *Deps, _ *planTestReader, _ *planTestWriter) { d.Writer = nil }, domain.ExitUsage},
	{"unchanged", func(_ *Deps, r *planTestReader, _ *planTestWriter) { r.items[0].Issue.Milestone = &r.milestones[0] }, domain.ExitOK},
}

func TestPlanMilestoneFailures(t *testing.T) {
	for _, tc := range planMilestoneFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, reader, writer, _ := planTestDeps(t)
			tc.configure(deps, reader, writer)
			err := planTestExecute(context.Background(), []string{"milestone", "set", "#7", "Release 1"}, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("code %v, error %v", domain.CodeOf(err), err)
			}
			if tc.code == domain.ExitDrift && len(writer.calls) != 0 {
				t.Fatal("write after drift")
			}
		})
	}
}

func TestPlanRetargetScope(t *testing.T) {
	for _, repository := range []string{"team/work", "other/work"} {
		t.Run(repository, func(t *testing.T) {
			_, reader, _, _ := planTestDeps(t)
			reader.items[0].Issue.Milestone = &reader.milestones[0]
			items, steps := planRetargetSteps(reader.items, repository, reader.milestones[0], reader.milestones[1])
			want := 0
			if repository == "team/work" {
				want = 1
			}
			if len(items) != want || len(steps) != want {
				t.Fatalf("steps = %+v", steps)
			}
		})
	}
}

var planRetargetErrors = []struct {
	name, from, to, failure string
	denied                  bool
	code                    domain.ExitCode
}{
	{"source", "Missing", "Release 2", "", false, domain.ExitNotFound},
	{"target", "Release 1", "Missing", "", false, domain.ExitNotFound},
	{"permission", "Release 1", "Release 2", "", true, domain.ExitPolicy},
	{"history", "Release 1", "Release 2", "milestones", false, domain.ExitAPI},
	{"list", "Release 1", "Release 2", "list", false, domain.ExitAPI},
}

func TestPlanRetargetErrors(t *testing.T) {
	for _, tc := range planRetargetErrors {
		t.Run(tc.name, func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			if tc.denied {
				reader.permission = domain.PermissionRead
			}
			if tc.failure != "" {
				reader.fail[tc.failure] = errors.New("offline")
			}
			err := planTestExecute(context.Background(), []string{"milestone", "retarget", tc.from, tc.to}, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatal(err)
			}
		})
	}
}
