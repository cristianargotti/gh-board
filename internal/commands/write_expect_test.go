package commands

import (
	"context"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var writeBuiltinCases = []struct {
	field domain.Field
	want  string
}{
	{domain.Field{Name: "People", DataType: domain.DataTypeAssignees}, "bob"},
	{domain.Field{Name: "Tags", DataType: domain.DataTypeLabels}, "old"},
	{domain.Field{Name: "Release", DataType: domain.DataTypeMilestone}, ""},
	{domain.Field{Name: "Summary", DataType: domain.DataTypeTitle}, "Task"},
	{domain.Field{Name: "Parent", DataType: domain.DataTypeParentIssue}, ""},
	{domain.Field{Name: "Kind", DataType: domain.DataTypeIssueType}, "Task"},
}

func TestWriteBuiltinExpectations(t *testing.T) {
	for _, tc := range writeBuiltinCases {
		t.Run(tc.field.Name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			fake.project.Fields = append(fake.project.Fields, tc.field)
			err := writeTestExecute(context.Background(), []string{"comment", "1", "text", "--expect", tc.field.Name + "=" + tc.want}, deps)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWriteGuardedExpectations(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(map[bool]string{false: "same", true: "drift"}[drift], func(t *testing.T) {
			writeAssertGuardedExpectation(t, drift)
		})
	}
}

func TestWritePlanConditionDimensions(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		steps int
	}{
		{[]string{"unassign", "1,2,3", "bob", "--expect", "assignees=bob"}, 3},
		{[]string{"label", "1,2,3", "-old", "--expect", "labels=old"}, 3},
		{[]string{"set", "1,2,3", "Start=2026-11-01", "--expect", "Start=2026-10-01"}, 6},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			deps, _, _ := writeFixture(t)
			deps.Config.Config.Capabilities.Dates.Source = domain.DateSourceIssueFields
			if err := writeTestExecute(context.Background(), tc.args, deps); domain.CodeOf(err) != domain.ExitPlanRequired {
				t.Fatal(err)
			}
			saved, err := plan.List(deps.Dirs.State)
			if err != nil || len(saved) != 1 || len(saved[0].Steps) != tc.steps {
				t.Fatalf("%+v: %v", saved, err)
			}
		})
	}
}

func TestWriteBuiltinExpectationAdvance(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	fake.project.Fields = append(fake.project.Fields, domain.Field{Name: "Tags", DataType: domain.DataTypeLabels})
	err := writeTestExecute(context.Background(), []string{"label", "1", "+bug", "-old", "--expect", "Tags=old"}, deps)
	if err != nil {
		t.Fatal(err)
	}
}

func writeAssertGuardedExpectation(t *testing.T, drift bool) {
	t.Helper()
	deps, fake, _ := writeFixture(t)
	args := []string{"move", "1", "Done", "--expect", "Epic=", "--expect", "Flow=Ready"}
	if err := writeTestExecute(context.Background(), args, deps); domain.CodeOf(err) != domain.ExitPlanRequired {
		t.Fatal(err)
	}
	saved, err := plan.List(deps.Dirs.State)
	if err != nil || len(saved) != 1 || len(saved[0].Steps) != 2 {
		t.Fatalf("%+v: %v", saved, err)
	}
	if drift {
		fake.items["I_kwDOTest0001"].Values["Epic"] = domain.FieldValue{Value: "Changed"}
	}
	_, err = plan.Apply(context.Background(), plan.Ports{Reader: fake, Writer: fake, Clock: deps.Clock}, saved[0],
		plan.ApplyOptions{Interactive: true, Viewer: fake.viewer, StateDir: deps.Dirs.State, Out: deps.Out})
	want := domain.ExitOK
	if drift {
		want = domain.ExitDrift
	}
	if domain.CodeOf(err) != want {
		t.Fatalf("%v", err)
	}
	if drift {
		writeAssertNoMutation(t, fake)
	}
}
