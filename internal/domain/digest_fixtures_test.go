package domain

import (
	"fmt"
	"time"
)

func digestTestTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}

func digestTestConfig() *Config {
	return &Config{Timezone: "America/Sao_Paulo", Capabilities: Capabilities{
		Status: &StatusCapability{Field: "Flow", Backlog: []string{"Queued"}, Ready: []string{"Ready"}, Active: []string{"Doing"}, Done: []string{"Finished"}},
		Triage: &TriageCapability{Label: "intake", DecisionField: "Decision", SLABusinessDays: 2, UrgentLabel: "rush"},
		Epic:   &EpicCapability{IssueType: "Feature", Field: "Theme"}, Task: &TaskCapability{IssueType: "Task"},
		Lane: &FieldCapability{Field: "Stream"}, Sprint: &FieldCapability{Field: "Cycle"},
		Dates: &DatesCapability{Start: "Begin", Target: "Due", Source: DateSourceProject}, Blocked: &FieldCapability{Field: "Obstacle"},
	}, Digest: Digest{Members: []string{"member"}, Metrics: []string{"first_response", "autonomy", "run_share"}}, Alerts: AlertRules{}}
}

func digestTestItem(number int) Item {
	return Item{ProjectItemID: fmt.Sprintf("item-%d", number), Issue: Issue{
		NodeID: fmt.Sprintf("issue-%d", number), Owner: "sample", Repo: "board", Number: number,
		Title: fmt.Sprintf("Work %d", number), State: IssueOpen, Labels: []Label{{Name: "intake"}},
		CreatedAt: digestTestTime("2026-10-05T12:00:00Z"), Assignees: []User{{Login: "member"}}, Type: &IssueType{Name: "Task"},
	}, Values: map[string]FieldValue{"Flow": {Value: "Doing", UpdatedAt: digestTestTime("2026-10-06T12:00:00Z")}}}
}

func digestTestInput() DigestInput {
	return DigestInput{
		Config: digestTestConfig(), Now: digestTestTime("2026-10-12T15:00:00Z"), Week: "2026-W41",
		Project: Project{Ref: ProjectRef{Owner: "sample", Number: 7}}, Timelines: map[string]IssueTimeline{},
	}
}

func tidyTestInput() TidyInput {
	cfg := digestTestConfig()
	fields := []Field{
		{Name: "Flow", DataType: DataTypeSingleSelect, Options: []FieldOption{{Name: "Queued"}, {Name: "Ready"}, {Name: "Doing"}, {Name: "Finished"}}},
		{Name: "Stream", DataType: DataTypeText},
		{Name: "Theme", DataType: DataTypeSingleSelect, Options: []FieldOption{{Name: "Platform"}}},
		{Name: "Begin", DataType: DataTypeDate},
		{Name: "Cycle", DataType: DataTypeIteration, Iterations: []Iteration{
			{Title: "Previous", Start: digestTestTime("2026-09-28T00:00:00Z"), Duration: 7, Completed: true},
			{Title: "Current", Start: digestTestTime("2026-10-05T00:00:00Z"), Duration: 7},
			{Title: "Next", Start: digestTestTime("2026-10-12T00:00:00Z"), Duration: 7},
		}},
	}
	return TidyInput{Config: cfg, Project: Project{Fields: fields}, Now: digestTestTime("2026-10-08T15:00:00Z")}
}

func tidyTestFamily() []Item {
	parent, child := digestTestItem(1), digestTestItem(2)
	parent.Issue.Type = &IssueType{Name: "Feature"}
	parent.Issue.SubIssues.Total = 1
	parent.Values["Stream"], parent.Values["Theme"] = FieldValue{Value: "Operations"}, FieldValue{Value: "Platform"}
	parent.Values["Flow"] = FieldValue{Value: "Queued"}
	child.Issue.Parent = &ParentRef{NodeID: parent.Issue.NodeID, Owner: "sample", Repo: "board", Number: 1}
	return []Item{parent, child}
}
