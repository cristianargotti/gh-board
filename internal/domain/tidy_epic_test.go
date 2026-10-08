package domain

import "testing"

var tidyEpicCases = []struct {
	name, after string
	statuses    []string
	edit        func(*TidyInput)
	skip        bool
}{
	{"active", "Doing", []string{"Doing"}, func(_ *TidyInput) {}, false},
	{"ready", "Ready", []string{"Ready"}, func(_ *TidyInput) {}, false},
	{"already backlog", "", []string{"Queued"}, func(_ *TidyInput) {}, false},
	{"return backlog", "Queued", []string{"Queued"}, func(in *TidyInput) { in.Items[0].Values["Flow"] = FieldValue{Value: "Ready"} }, false},
	{"all done", "Finished", []string{"Finished", "Finished"}, func(_ *TidyInput) {}, false},
	{"mixed done", "Doing", []string{"Finished", "Queued"}, func(_ *TidyInput) {}, false},
	{"mixed ready", "Ready", []string{"Ready", "Queued"}, func(_ *TidyInput) {}, false},
	{"closed task overrides status", "Finished", []string{"Doing"}, func(in *TidyInput) { in.Items[1].Issue.State = IssueClosed }, false},
	{"unmapped status", "", []string{"Mystery"}, func(_ *TidyInput) {}, true},
	{"missing tasks", "", nil, func(_ *TidyInput) {}, true},
	{"partial children", "", []string{"Finished"}, func(in *TidyInput) { in.Items[0].Issue.SubIssues.Total = 3 }, true},
	{"no status mapping", "", []string{"Doing"}, func(in *TidyInput) { in.Config.Capabilities.Status = nil }, true},
	{"ambiguous active", "", []string{"Doing"}, func(in *TidyInput) { in.Config.Capabilities.Status.Active = []string{"Doing", "Review"} }, true},
	{"transition refused", "", []string{"Doing"}, func(in *TidyInput) { in.Config.Policy.Transitions = map[string][]string{"Queued": {"Ready"}} }, true},
	{"wip refused", "", []string{"Doing"}, func(in *TidyInput) { in.Config.Policy.WIP = map[string]int{"Doing": 1} }, true},
	{"non task children", "", []string{"Doing"}, func(in *TidyInput) { in.Items[1].Issue.Type = &IssueType{Name: "Bug"} }, true},
	{"no task mapping", "", []string{"Doing"}, func(in *TidyInput) { in.Config.Capabilities.Task = nil }, true},
	{"no target option", "", []string{"Doing"}, func(in *TidyInput) { in.Project.Fields[0].Options = nil }, true},
	{"unchanged active", "", []string{"Doing"}, func(in *TidyInput) { in.Items[0].Values["Flow"] = FieldValue{Value: "Doing"} }, false},
}

func tidyEpicInput(statuses []string) TidyInput {
	in := tidyTestInput()
	in.Config.Tidy.EpicFollowsTasks = true
	in.Items = tidyTestFamily()[:1]
	in.Items[0].Issue.SubIssues.Total = len(statuses)
	for i, status := range statuses {
		child := digestTestItem(i + 2)
		child.Issue.Parent = &ParentRef{NodeID: in.Items[0].Issue.NodeID}
		child.Values["Flow"] = FieldValue{Value: status}
		in.Items = append(in.Items, child)
	}
	return in
}

func TestTidyEpic(t *testing.T) {
	for _, tc := range tidyEpicCases {
		t.Run(tc.name, func(t *testing.T) {
			in := tidyEpicInput(tc.statuses)
			tc.edit(&in)
			out := BuildTidyPlan(in)
			if (len(out.Steps) == 1) != (tc.after != "") || (len(out.Skipped) > 0) != tc.skip {
				t.Fatalf("tidy = %+v", out)
			}
			if tc.after != "" && (out.Steps[0].After != tc.after || out.Steps[0].Before != in.Items[0].Text("Flow")) {
				t.Fatalf("step = %+v", out.Steps[0])
			}
		})
	}
}

func TestTidyProjectedWIP(t *testing.T) {
	for _, limit := range []int{3, 4} {
		in := tidyEpicInput([]string{"Doing"})
		other := tidyEpicInput([]string{"Doing"})
		other.Items[0].Issue.NodeID, other.Items[1].Issue.NodeID = "issue-3", "issue-4"
		other.Items[1].Issue.Parent.NodeID = "issue-3"
		in.Items = append(in.Items, other.Items...)
		in.Config.Policy.WIP = map[string]int{"Doing": limit}
		out := BuildTidyPlan(in)
		if len(out.Steps) != limit-2 {
			t.Fatalf("limit %d, steps = %+v", limit, out.Steps)
		}
	}
}
