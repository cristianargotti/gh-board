package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var tidyInheritCases = []struct {
	name         string
	edit         func(*TidyInput)
	steps, skips int
}{
	{"both roles", func(_ *TidyInput) {}, 2, 0},
	{"already inherited", tidyTestInherited, 0, 0},
	{"no config", func(in *TidyInput) { in.Config = nil }, 0, 0},
	{"disabled", func(in *TidyInput) { in.Config.Tidy.InheritFromParent = nil }, 0, 0},
	{"orphan", func(in *TidyInput) { in.Items = in.Items[1:] }, 0, 1},
	{"not epic", func(in *TidyInput) { in.Items[0].Issue.Type = nil }, 0, 1},
	{"missing mapping", func(in *TidyInput) { in.Config.Capabilities.Lane = nil }, 1, 1},
	{"missing parent value", func(in *TidyInput) { delete(in.Items[0].Values, "Theme") }, 1, 1},
	{"missing schema", func(in *TidyInput) { in.Project.Fields = nil }, 0, 2},
	{"unknown option", func(in *TidyInput) { in.Items[0].Values["Theme"] = FieldValue{Value: "Unknown"} }, 1, 1},
	{"closed child", func(in *TidyInput) { in.Items[1].Issue.State = IssueClosed }, 0, 0},
	{"archived child", func(in *TidyInput) { in.Items[1].Archived = true }, 0, 0},
	{"reference fallback", func(in *TidyInput) { in.Items[1].Issue.Parent.NodeID = "" }, 2, 0},
	{"node ID authoritative", func(in *TidyInput) { in.Items[1].Issue.Parent.NodeID = "wrong" }, 0, 1},
	{"unknown role", func(in *TidyInput) { in.Config.Tidy.InheritFromParent = []string{"unknown"} }, 0, 1},
}

func tidyTestInherited(in *TidyInput) {
	in.Items[1].Values["Theme"] = in.Items[0].Values["Theme"]
	in.Items[1].Values["Stream"] = in.Items[0].Values["Stream"]
}

func TestTidyInheritance(t *testing.T) {
	for _, tc := range tidyInheritCases {
		t.Run(tc.name, func(t *testing.T) {
			in := tidyTestInput()
			in.Config.Tidy.InheritFromParent, in.Items = []string{"lane", "epic"}, tidyTestFamily()
			tc.edit(&in)
			before, _ := json.Marshal(in)
			out := BuildTidyPlan(in)
			after, _ := json.Marshal(in)
			if len(out.Steps) != tc.steps || len(out.Skipped) != tc.skips || !reflect.DeepEqual(before, after) {
				t.Fatalf("tidy = %+v, mutated = %t", out, !reflect.DeepEqual(before, after))
			}
			for i, step := range out.Steps {
				if step.Index != i || step.Before != "" || step.After == "" || step.Target.NodeID != "issue-2" || !strings.Contains(step.Description, "épico pai") {
					t.Fatalf("step = %+v", step)
				}
			}
		})
	}
}

var tidyFieldCases = []struct {
	field Field
	value string
	want  bool
}{
	{Field{DataType: DataTypeText}, "value", true},
	{Field{DataType: DataTypeDate}, "2026-10-01", true},
	{Field{DataType: DataTypeSingleSelect, Options: []FieldOption{{Name: "Present"}}}, "Present", true},
	{Field{DataType: DataTypeSingleSelect}, "Absent", false},
	{Field{DataType: DataTypeIteration, Iterations: []Iteration{{Title: "Present"}}}, "Present", true},
	{Field{DataType: DataTypeIteration}, "Absent", false},
	{Field{DataType: DataTypeNumber}, "1", false},
}

func TestTidyFieldValidation(t *testing.T) {
	for _, tc := range tidyFieldCases {
		if got := tidyAccepts(tc.field, tc.value); got != tc.want {
			t.Errorf("accepts(%+v, %q) = %t", tc.field, tc.value, got)
		}
	}
}

func TestTidyBeforeValues(t *testing.T) {
	for _, before := range []string{"Legacy", "  Legacy  "} {
		in := tidyTestInput()
		in.Config.Tidy.InheritFromParent = []string{"lane"}
		in.Items = tidyTestFamily()
		in.Items[1].Values["Stream"] = FieldValue{Value: before}
		out := BuildTidyPlan(in)
		if len(out.Steps) != 1 || out.Steps[0].Before != before || out.Steps[0].After != "Operations" {
			t.Fatalf("steps = %+v", out.Steps)
		}
		if value, _ := StepCurrentValue(out.Steps[0], in.Items[1]); value != before {
			t.Fatal("incompatible drift encoding")
		}
	}
}
