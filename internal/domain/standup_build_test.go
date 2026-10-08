package domain

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

var standupCases = []struct {
	name                   string
	edit                   func(*StandupInput)
	moved, active, blocked int
	people                 int
}{
	{"active moved", func(_ *StandupInput) {}, 1, 1, 0, 1},
	{"issue update is not movement", func(in *StandupInput) { in.Items[0].Values["Flow"] = FieldValue{Value: "Doing"} }, 0, 1, 0, 1},
	{"closed moved", func(in *StandupInput) { in.Items[0].Issue.State = IssueClosed }, 1, 0, 0, 1},
	{"closed not moved", standupTestClosed, 0, 0, 0, 0},
	{"archived", func(in *StandupInput) { in.Items[0].Archived = true }, 0, 0, 0, 0},
	{"native blocked", func(in *StandupInput) { in.Items[0].Issue.BlockedByCount = 1 }, 1, 1, 1, 1},
	{"mapped blocked", func(in *StandupInput) { in.Items[0].Values["Obstacle"] = FieldValue{Value: "Dependency"} }, 1, 1, 1, 1},
	{"blank blocker", func(in *StandupInput) { in.Items[0].Values["Obstacle"] = FieldValue{Value: " "} }, 1, 1, 0, 1},
	{"matching login", func(in *StandupInput) { in.For = "MEMBER" }, 1, 1, 0, 1},
	{"different login", func(in *StandupInput) { in.For = "other" }, 0, 0, 0, 1},
	{"no assignee", func(in *StandupInput) { in.Items[0].Issue.Assignees = nil }, 0, 0, 0, 0},
	{"generic native blockers", standupTestGeneric, 0, 0, 1, 1},
	{"local yesterday boundary", func(in *StandupInput) {
		in.Items[0].Values["Flow"] = FieldValue{Value: "Doing", UpdatedAt: digestTestTime("2026-10-08T02:59:59Z")}
	}, 1, 1, 0, 1},
	{"local today boundary", func(in *StandupInput) {
		in.Items[0].Values["Flow"] = FieldValue{Value: "Doing", UpdatedAt: digestTestTime("2026-10-08T03:00:00Z")}
	}, 0, 1, 0, 1},
}

func standupTestClosed(in *StandupInput) {
	in.Items[0].Issue.State = IssueClosed
	in.Items[0].Values["Flow"] = FieldValue{Value: "Finished", UpdatedAt: time.Time{}}
}

func standupTestGeneric(in *StandupInput) {
	in.Config = nil
	in.Items[0].Issue.BlockedByCount = 1
}

func TestBuildStandup(t *testing.T) {
	for _, tc := range standupCases {
		t.Run(tc.name, func(t *testing.T) {
			in := StandupInput{Config: digestTestConfig(), Items: []Item{digestTestItem(1)}, Now: digestTestTime("2026-10-08T15:00:00Z")}
			in.Items[0].Values["Flow"] = FieldValue{Value: "Doing", UpdatedAt: digestTestTime("2026-10-07T15:00:00Z")}
			tc.edit(&in)
			before, _ := json.Marshal(in)
			out := BuildStandup(in)
			after, _ := json.Marshal(in)
			if len(out.People) != tc.people || !reflect.DeepEqual(before, after) {
				t.Fatalf("standup = %+v, input changed = %t", out, !reflect.DeepEqual(before, after))
			}
			if tc.people > 0 {
				p := out.People[0]
				if len(p.MovedYesterday) != tc.moved || len(p.ActiveToday) != tc.active || len(p.Blocked) != tc.blocked {
					t.Fatalf("person = %+v", p)
				}
			}
		})
	}
}

var standupAssigneeCases = []struct {
	users []User
	want  []string
}{
	{[]User{{Login: "zeta"}, {Login: "alpha"}, {Login: "ALPHA"}, {}}, []string{"alpha", "zeta"}},
	{[]User{{Login: "member"}, {Login: "member"}}, []string{"member"}},
	{nil, []string{}},
}

func TestStandupMultipleAssignees(t *testing.T) {
	for _, tc := range standupAssigneeCases {
		in := StandupInput{Config: digestTestConfig(), Items: []Item{digestTestItem(1)}, Now: digestTestTime("2026-10-08T15:00:00Z")}
		in.Items[0].Issue.Assignees = tc.users
		out, got := BuildStandup(in), []string{}
		for _, person := range out.People {
			got = append(got, person.Login)
			if len(person.ActiveToday) != 1 {
				t.Fatalf("duplicates: %+v", person)
			}
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("people = %v, want %v", got, tc.want)
		}
	}
}
