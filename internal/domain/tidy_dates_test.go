package domain

import (
	"strings"
	"testing"
	"time"
)

var tidySprintCases = []struct {
	name, target, after string
	edit                func(*TidyInput)
	skip                bool
}{
	{"current", "2026-10-05", "Current", func(_ *TidyInput) {}, false},
	{"completed", "2026-10-04", "Previous", func(_ *TidyInput) {}, false},
	{"next boundary", "2026-10-12", "Next", func(_ *TidyInput) {}, false},
	{"outside", "2026-10-19", "", func(_ *TidyInput) {}, true},
	{"invalid", "broken", "", func(_ *TidyInput) {}, true},
	{"missing", "", "", func(_ *TidyInput) {}, true},
	{"no dates mapping", "2026-10-05", "", func(in *TidyInput) { in.Config.Capabilities.Dates = nil }, true},
	{"no sprint mapping", "2026-10-05", "", func(in *TidyInput) { in.Config.Capabilities.Sprint = nil }, true},
	{"schema absent", "2026-10-05", "", func(in *TidyInput) { in.Project.Fields = nil }, true},
	{"wrong field type", "2026-10-05", "", func(in *TidyInput) { in.Project.Fields[4].DataType = DataTypeText }, true},
	{"overlap", "2026-10-05", "", func(in *TidyInput) { in.Project.Fields[4].Iterations[0].Duration = 14 }, true},
	{"already correct", "2026-10-05", "", func(in *TidyInput) { in.Items[0].Values["Cycle"] = FieldValue{Value: "Current"} }, false},
	{"issue date source", "2026-10-12", "Next", func(in *TidyInput) { in.Config.Capabilities.Dates.Source = DateSourceIssueFields }, false},
}

func TestTidySprint(t *testing.T) {
	for _, tc := range tidySprintCases {
		t.Run(tc.name, func(t *testing.T) {
			in := tidyTestInput()
			in.Config.Tidy.SprintFromTarget = true
			in.Items = []Item{digestTestItem(1)}
			in.Items[0].Values["Due"] = FieldValue{Value: tc.target}
			in.Items[0].IssueFields = []IssueFieldValue{{Name: "Due", Value: tc.target}}
			tc.edit(&in)
			out := BuildTidyPlan(in)
			if (len(out.Skipped) > 0) != tc.skip || (len(out.Steps) == 1) != (tc.after != "") {
				t.Fatalf("tidy = %+v", out)
			}
			if tc.after != "" && (out.Steps[0].After != tc.after || out.Steps[0].Field != "Cycle") {
				t.Fatalf("step = %+v", out.Steps[0])
			}
		})
	}
}

var tidyStartCases = []struct {
	name         string
	edit         func(*TidyInput)
	op           Operation
	steps, skips int
}{
	{"project date", func(_ *TidyInput) {}, OpSetFieldValue, 1, 0},
	{"issue date", func(in *TidyInput) { in.Config.Capabilities.Dates.Source = DateSourceIssueFields }, OpSetIssueFieldValue, 1, 0},
	{"existing start", func(in *TidyInput) { in.Items[0].Values["Begin"] = FieldValue{Value: "2026-09-01"} }, "", 0, 0},
	{"existing malformed start", func(in *TidyInput) { in.Items[0].Values["Begin"] = FieldValue{Value: "malformed"} }, "", 0, 0},
	{"missing timestamp", func(in *TidyInput) { in.Items[0].Values["Flow"] = FieldValue{Value: "Doing"} }, "", 0, 1},
	{"future timestamp", func(in *TidyInput) {
		in.Items[0].Values["Flow"] = FieldValue{Value: "Doing", UpdatedAt: in.Now.Add(time.Hour)}
	}, "", 0, 1},
	{"missing now", func(in *TidyInput) { in.Now = time.Time{} }, "", 0, 1},
	{"inactive", func(in *TidyInput) { in.Items[0].Values["Flow"] = FieldValue{Value: "Ready"} }, "", 0, 0},
	{"no date mapping", func(in *TidyInput) { in.Config.Capabilities.Dates = nil }, "", 0, 1},
	{"no start mapping", func(in *TidyInput) { in.Config.Capabilities.Dates.Start = "" }, "", 0, 1},
}

func TestTidyStart(t *testing.T) {
	for _, tc := range tidyStartCases {
		t.Run(tc.name, func(t *testing.T) {
			in := tidyTestInput()
			in.Config.Tidy.StampStartOnActive, in.Items = true, []Item{digestTestItem(1)}
			in.Items[0].Values["Flow"] = FieldValue{Value: "Doing", UpdatedAt: digestTestTime("2026-10-07T02:00:00Z")}
			tc.edit(&in)
			out := BuildTidyPlan(in)
			if len(out.Steps) != tc.steps || len(out.Skipped) != tc.skips {
				t.Fatalf("tidy = %+v", out)
			}
			if tc.steps == 0 {
				return
			}
			tidyCheckStart(t, out.Steps[0], tc.op)
		})
	}
}

func tidyCheckStart(t *testing.T, step Step, op Operation) {
	t.Helper()
	if step.Operation != op || step.Before != "" || step.After != "2026-10-06" {
		t.Fatalf("step = %+v", step)
	}
	if op == OpSetIssueFieldValue && !strings.Contains(step.Description, "todos os projetos") {
		t.Fatal("missing organization field impact")
	}
}
