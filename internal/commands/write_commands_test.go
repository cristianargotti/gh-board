package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var writeSuccessCases = []struct {
	name     string
	args     []string
	mutation string
}{
	{"move", []string{"move", "1", "Active"}, "setField"},
	{"set", []string{"set", "1", "Epic=Revenue", "Size=2"}, "setField"},
	{"status set", []string{"set", "1", "Flow=Active"}, "setField"},
	{"assign viewer", []string{"assign", "1", "alice"}, "assign"},
	{"assign external", []string{"assign", "1", "carol"}, "assign"},
	{"assign known", []string{"assign", "1", "bob"}, "assign"},
	{"unassign", []string{"unassign", "1", "bob"}, "unassign"},
	{"sprint current", []string{"sprint", "set", "1", "current"}, "setField"},
	{"sprint next", []string{"sprint", "set", "1", "next"}, "setField"},
	{"sprint title", []string{"sprint", "set", "1", "Cycle 2"}, "setField"},
	{"estimate", []string{"estimate", "1", "2.5"}, "setField"},
	{"dates", []string{"dates", "1", "--start", "2026-10-08", "--target", "2026-10-09"}, "setField"},
	{"milestone", []string{"milestone", "set", "1", "Release"}, "updateIssue"},
	{"comment", []string{"comment", "1", "Review complete"}, "comment"},
	{"link", []string{"link", "1", "--parent", "2"}, "link"},
	{"label", []string{"label", "1", "+bug", "-old", "--reason", "cleanup"}, "removeLabels"},
	{"reopen", []string{"reopen", "1"}, "reopen"},
	{"restore", []string{"restore", "1"}, "restore"},
	{"new task", []string{"new", "task", "--title", "Task"}, "create"},
	{"new epic", []string{"new", "epic", "--title", "Feature"}, "create"},
	{"new entry", []string{"new", "entry", "--title", "Incoming"}, "create"},
	{"duplicate ref", []string{"estimate", "1,1", "1"}, "setField"},
}

func TestWriteCommands(t *testing.T) {
	for _, tc := range writeSuccessCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			if tc.name == "reopen" {
				it := fake.items["I_kwDOTest0001"]
				it.Issue.State = domain.IssueClosed
				fake.items["I_kwDOTest0001"] = it
			}
			if tc.name == "restore" {
				it := fake.items["I_kwDOTest0001"]
				it.Archived = true
				fake.items["I_kwDOTest0001"] = it
			}
			if err := writeTestExecute(context.Background(), tc.args, deps); err != nil {
				t.Fatal(err)
			}
			if !domain.Contains(fake.calls, tc.mutation) {
				t.Fatalf("missing mutation %s: %v", tc.mutation, fake.calls)
			}
			writeAssertRecorded(t, deps, fake)
		})
	}
}

func writeAssertRecorded(t *testing.T, deps *Deps, fake *writeFake) {
	t.Helper()
	entries, err := audit.Query(deps.Dirs.State, time.Time{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("audit: %v, %v", entries, err)
	}
	if entries[0].Actor != fake.viewer.Login || entries[0].Result != "ok" {
		t.Fatalf("audit: %+v", entries[0])
	}
	p, err := plan.Read(deps.Dirs.State, entries[0].PlanID)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := plan.NewJournal(deps.Dirs.State).Read(p.ID)
	if err != nil || len(journal) != len(p.Steps) {
		t.Fatalf("journal=%+v, steps=%d, err=%v", journal, len(p.Steps), err)
	}
	for _, entry := range journal {
		if entry.Status != plan.StatusDone {
			t.Fatalf("entry: %+v", entry)
		}
	}
}

func TestWriteNewAllFlags(t *testing.T) {
	for _, source := range []domain.DateSource{domain.DateSourceProject, domain.DateSourceIssueFields} {
		t.Run(string(source), func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			deps.Config.Config.Capabilities.Dates.Source = source
			args := []string{
				"new", "task", "--title", "My task", "--body", "Details", "--parent", "2", "--lane", "Platform",
				"--epic", "Revenue", "--sprint", "current", "--estimate", "2", "--start", "2026-10-08", "--target", "2026-10-09",
				"--milestone", "Release", "--assignee", "alice", "--label", "bug", "--reason", "work",
			}
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
			if fake.created.IssueTypeID != "T_task" || fake.created.MilestoneID != "M_one" || fake.created.Body != "Details" {
				t.Fatalf("input: %+v", fake.created)
			}
			if len(fake.created.AssigneeIDs) != 1 || len(fake.created.LabelIDs) != 1 {
				t.Fatalf("input: %+v", fake.created)
			}
			writeAssertRecorded(t, deps, fake)
		})
	}
}

var writePlanCases = [][]string{
	{"move", "1", "Done"},
	{"set", "1", "Flow=Done"},
	{"estimate", "1,2,3", "1"},
	{"milestone", "set", "1,2,3", "Release"},
}

func TestWritePlans(t *testing.T) {
	for _, args := range writePlanCases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			deps, fake, out := writeFixture(t)
			err := writeTestExecute(context.Background(), args, deps)
			if domain.CodeOf(err) != domain.ExitPlanRequired {
				t.Fatalf("%v", err)
			}
			writeAssertPlan(t, deps, fake, out.String(), err)
		})
	}
}

func TestWriteDryRuns(t *testing.T) {
	for _, tc := range writeSuccessCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			args := append(append([]string{}, tc.args...), "--dry-run", "--json")
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
			if domain.Contains(fake.calls, tc.mutation) {
				t.Fatalf("dry run mutated: %v", fake.calls)
			}
			plans, err := plan.List(deps.Dirs.State)
			if err != nil || len(plans) != 0 {
				t.Fatalf("dry run saved plan: %v, %v", plans, err)
			}
			entries, err := audit.Query(deps.Dirs.State, time.Time{})
			if err != nil || len(entries) != 1 || !entries[0].DryRun {
				t.Fatalf("audit: %v, %v", entries, err)
			}
		})
	}
}

func writeAssertPlan(t *testing.T, deps *Deps, fake *writeFake, output string, err error) {
	t.Helper()
	plans, readErr := plan.List(deps.Dirs.State)
	if readErr != nil || len(plans) != 1 {
		t.Fatalf("plans %v: %v", plans, readErr)
	}
	command := "gh board apply " + plans[0].ID
	if !strings.Contains(err.Error(), command) || !strings.Contains(output, command) {
		t.Fatalf("missing %q", command)
	}
	writeAssertNoMutation(t, fake)
	step := plans[0].Steps[0]
	if step.Field == "Flow" && step.Before != "Ready" {
		t.Fatalf("missing status condition: %+v", step)
	}
}
