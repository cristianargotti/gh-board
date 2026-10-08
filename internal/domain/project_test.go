package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var projectRefCases = []struct {
	in   string
	want domain.ProjectRef
	ok   bool
}{
	{"acme/7", domain.ProjectRef{Owner: "acme", Number: 7}, true},
	{" ana/2 ", domain.ProjectRef{Owner: "ana", Number: 2}, true},
	{"acme", domain.ProjectRef{}, false},
	{"/7", domain.ProjectRef{}, false},
	{"acme/0", domain.ProjectRef{}, false},
	{"acme/abc", domain.ProjectRef{}, false},
	{"a/b/7", domain.ProjectRef{}, false},
}

func TestParseProjectRef(t *testing.T) {
	for _, tc := range projectRefCases {
		got, err := domain.ParseProjectRef(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("%q: got %+v, %v", tc.in, got, err)
		}
		if !tc.ok && !errors.Is(err, domain.ErrUsage) {
			t.Errorf("%q: expected ErrUsage, got %v", tc.in, err)
		}
	}
	if s := (domain.ProjectRef{Owner: "o", Number: 3}).String(); s != "o/3" {
		t.Errorf("String() = %q", s)
	}
	if !(domain.ProjectRef{}).IsZero() || (domain.ProjectRef{Owner: "o"}).IsZero() {
		t.Error("IsZero is wrong")
	}
}

var sprintStart = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

var sampleProject = domain.Project{
	Fields: []domain.Field{
		{ID: "f1", Name: "Status", DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{{ID: "o1", Name: "BACKLOG"}, {ID: "o2", Name: "DONE"}}},
		{ID: "f2", Name: "Sprint", DataType: domain.DataTypeIteration, Iterations: []domain.Iteration{
			{ID: "i1", Title: "Sprint 4", Start: sprintStart.AddDate(0, 0, -14), Duration: 14, Completed: true},
			{ID: "i2", Title: "Sprint 5", Start: sprintStart, Duration: 14},
		}},
	},
	Repositories: []domain.Repository{{Owner: "acme", Name: "app"}},
}

func TestProjectLookups(t *testing.T) {
	status, ok := sampleProject.FieldByName("Status")
	if !ok || status.ID != "f1" {
		t.Fatalf("FieldByName = %+v, %v", status, ok)
	}
	if _, ok := sampleProject.FieldByName("Missing"); ok {
		t.Fatal("missing field must not be found")
	}
	if names := sampleProject.FieldNames(); len(names) != 2 || names[1] != "Sprint" {
		t.Fatalf("FieldNames = %v", names)
	}
	done, ok := status.OptionByName("DONE")
	if !ok || done.ID != "o2" {
		t.Fatalf("OptionByName = %+v, %v", done, ok)
	}
	if _, ok := status.OptionByName("nope"); ok {
		t.Fatal("missing option must not be found")
	}
	if names := status.OptionNames(); len(names) != 2 || names[0] != "BACKLOG" {
		t.Fatalf("OptionNames = %v", names)
	}
	if full := sampleProject.Repositories[0].FullName(); full != "acme/app" {
		t.Fatalf("FullName = %q", full)
	}
}

func TestIterations(t *testing.T) {
	sprint, _ := sampleProject.FieldByName("Sprint")
	it, ok := sprint.IterationByTitle("Sprint 5")
	if !ok || it.ID != "i2" {
		t.Fatalf("IterationByTitle = %+v, %v", it, ok)
	}
	if _, ok := sprint.IterationByTitle("Sprint 9"); ok {
		t.Fatal("missing iteration must not be found")
	}
	current, ok := sprint.IterationAt(sprintStart.AddDate(0, 0, 3))
	if !ok || current.Title != "Sprint 5" {
		t.Fatalf("IterationAt = %+v, %v", current, ok)
	}
	if _, ok := sprint.IterationAt(sprintStart.AddDate(0, 0, 30)); ok {
		t.Fatal("no iteration covers a far future date")
	}
	if end := it.End(); !end.Equal(sprintStart.AddDate(0, 0, 14)) {
		t.Fatalf("End = %v", end)
	}
	if it.Contains(it.End()) || !it.Contains(it.Start) {
		t.Fatal("Contains must be start-inclusive and end-exclusive")
	}
	if left := it.DaysLeft(sprintStart.AddDate(0, 0, 10)); left != 3 {
		t.Fatalf("DaysLeft = %d", left)
	}
	if last := it.LastDay(); !last.Equal(sprintStart.AddDate(0, 0, 13)) || it.DaysLeft(last) != 0 {
		t.Fatalf("LastDay = %v, DaysLeft on it = %d", last, it.DaysLeft(last))
	}
	if left := it.DaysLeft(sprintStart.AddDate(0, 0, 20)); left != 0 {
		t.Fatalf("DaysLeft past the end = %d", left)
	}
}
