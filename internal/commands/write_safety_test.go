package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestWriteLastRead(t *testing.T) {
	for _, mutation := range []string{"comment", "assign", "setField"} {
		t.Run(mutation, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			args := map[string][]string{"comment": {"comment", "1", "text"}, "assign": {"assign", "1", "alice"}, "setField": {"move", "1", "Active"}}
			if err := writeTestExecute(context.Background(), args[mutation], deps); err != nil {
				t.Fatal(err)
			}
			for i, call := range fake.calls {
				if call == mutation && (i == 0 || fake.calls[i-1] != "read") {
					t.Fatalf("no fresh read: %v", fake.calls)
				}
			}
		})
	}
}

func TestWriteExpectAcrossSteps(t *testing.T) {
	for _, args := range [][]string{
		{"set", "1", "Flow=Active", "Size=2", "--expect", "Flow=Ready"},
		{"label", "1", "+bug", "-old", "--expect", "labels=old"},
		{"dates", "1", "--start", "2026-10-08", "--target", "2026-10-09", "--expect", "Start=2026-10-01"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			deps, _, _ := writeFixture(t)
			deps.Config.Config.Capabilities.Dates.Source = domain.DateSourceIssueFields
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWritePartialFailure(t *testing.T) {
	for _, mutation := range []string{"addProject", "setField"} {
		t.Run(mutation, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			fake.failure = mutation
			err := writeTestExecute(context.Background(), []string{"new", "task", "--title", "T", "--estimate", "1"}, deps)
			if domain.CodeOf(err) != domain.ExitAPI {
				t.Fatalf("%v", err)
			}
			writeAssertPartialFailure(t, deps, mutation)
		})
	}
}

var writeRereadCases = []struct {
	name   string
	read   int
	change func(*writeFake)
	code   domain.ExitCode
}{
	{"missing", 2, nil, 2},
	{"item replaced", 3, func(f *writeFake) {
		it := f.items["I_kwDOTest0001"]
		it.ProjectItemID = "PVTI_other"
		f.items["I_kwDOTest0001"] = it
	}, 4},
	{"expect changed", 3, func(f *writeFake) { writeSetStatus(f, "I_kwDOTest0001", "Done") }, 4},
}

func TestWriteRereadRefusals(t *testing.T) {
	for _, tc := range writeRereadCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			fake.changeAt, fake.change = tc.read, tc.change
			if tc.change == nil {
				fake.failAt = tc.read
			}
			err := writeTestExecute(context.Background(), []string{"comment", "1", "Hello", "--expect", "Flow=Ready"}, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("%v", err)
			}
			writeAssertNoMutation(t, fake)
		})
	}
}

func TestWritePagination(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "complete", false: "incomplete"}[valid], func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			page := domain.ItemPage{Items: []domain.Item{writeItem(2)}, HasNext: true}
			if valid {
				page.NextCursor = "next"
			}
			fake.listPages = []domain.ItemPage{page, {Items: []domain.Item{writeItem(3)}}}
			err := writeTestExecute(context.Background(), []string{"move", "1", "Active"}, deps)
			if (err == nil) != valid {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestWriteIterationOrdering(t *testing.T) {
	now := writeClock{}.Now()
	field := domain.Field{Iterations: []domain.Iteration{
		{ID: "complete", Start: now.Add(-time.Hour), Duration: 7, Completed: true},
		{ID: "later", Start: now.AddDate(0, 0, 5), Duration: 7},
		{ID: "sooner", Start: now.AddDate(0, 0, 1), Duration: 7},
	}}
	iteration, err := writeIteration(field, "next", now)
	if err != nil || iteration.ID != "sooner" {
		t.Fatalf("%+v: %v", iteration, err)
	}
}

func writeAssertPartialFailure(t *testing.T, deps *Deps, mutation string) {
	t.Helper()
	plans, err := plan.List(deps.Dirs.State)
	if err != nil || len(plans) != 1 {
		t.Fatalf("%v: %+v", err, plans)
	}
	entries, err := plan.NewJournal(deps.Dirs.State).Read(plans[0].ID)
	if err != nil || len(entries) < 2 {
		t.Fatalf("%v: %+v", err, entries)
	}
	if entries[0].Status != plan.StatusDone || entries[len(entries)-1].Status != plan.StatusFailed {
		t.Fatalf("%+v", entries)
	}
	if mutation == "setField" && entries[1].Created != "PVTI_new" {
		t.Fatal("project item resume binding missing")
	}
	if plans[0].Steps[1].Target.NodeID != plan.Ref(0) || plans[0].Steps[2].Target.ProjectItemID != plan.Ref(1) {
		t.Fatal("creation references missing")
	}
}
