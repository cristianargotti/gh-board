package plan_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestJournalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	journal := plan.NewJournal(dir)
	if journal.Dir() != dir {
		t.Fatal("Dir is wrong")
	}
	entries := []plan.JournalEntry{
		{PlanID: "p1", Step: 0, Operation: domain.OpCreateIssue, Status: plan.StatusDone, Created: "I_new", At: now},
		{PlanID: "p1", Step: 1, Operation: domain.OpAddComment, Status: plan.StatusFailed, Error: "boom", At: now},
	}
	for _, e := range entries {
		if err := journal.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := audit.AppendLine(dir+"/journal/p1.jsonl", []byte("{broken")); err != nil {
		t.Fatal(err)
	}
	got, err := journal.Read("p1")
	if err != nil || len(got) != 2 || got[0].Created != "I_new" || got[1].Error != "boom" {
		t.Fatalf("Read = %+v, %v", got, err)
	}
	if got, err := journal.Read("p2"); err != nil || len(got) != 0 {
		t.Fatalf("missing journal = %v, %v", got, err)
	}
}

func TestJournalInvalidID(t *testing.T) {
	journal := plan.NewJournal(t.TempDir())
	if err := journal.Append(plan.JournalEntry{PlanID: "../x"}); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("Append: %v", err)
	}
	if _, err := journal.Read(""); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("Read: %v", err)
	}
	if _, err := plan.NewJournal(t.TempDir() + "/journal").Read("p1"); err != nil {
		t.Fatalf("missing dir: %v", err)
	}
	blocker := t.TempDir()
	writeFile(t, blocker+"/journal", []byte("x"))
	if err := plan.NewJournal(blocker).Append(plan.JournalEntry{PlanID: "p1"}); err == nil {
		t.Fatal("a journal directory under a file must fail")
	}
	if _, err := plan.NewJournal(blocker).Read("p1"); err == nil {
		t.Fatal("reading through a file must fail")
	}
}

var doneStepsCases = []struct {
	name    string
	entries []plan.JournalEntry
	want    []int
}{
	{name: "empty", entries: nil, want: nil},
	{name: "done and skipped count", entries: []plan.JournalEntry{{Step: 0, Status: plan.StatusDone}, {Step: 1, Status: plan.StatusSkipped}}, want: []int{0, 1}},
	{name: "failed does not", entries: []plan.JournalEntry{{Step: 0, Status: plan.StatusFailed}}, want: nil},
	{name: "failed after done keeps done", entries: []plan.JournalEntry{{Step: 0, Status: plan.StatusDone}, {Step: 0, Status: plan.StatusFailed}}, want: []int{0}},
}

func TestDoneSteps(t *testing.T) {
	for _, tc := range doneStepsCases {
		t.Run(tc.name, func(t *testing.T) {
			done := plan.DoneSteps(tc.entries)
			if len(done) != len(tc.want) {
				t.Fatalf("DoneSteps = %v, want %v", done, tc.want)
			}
			for _, i := range tc.want {
				if _, ok := done[i]; !ok {
					t.Fatalf("step %d missing from %v", i, done)
				}
			}
		})
	}
}
