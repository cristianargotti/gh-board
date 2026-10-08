package domain

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

var digestWeekCases = []struct {
	week, start string
	wantErr     bool
}{
	{"2026-W41", "2026-10-05T03:00:00Z", false},
	{"", "2026-10-12T03:00:00Z", false},
	{"2020-W53", "2020-12-28T03:00:00Z", false},
	{"2021-W01", "2021-01-04T03:00:00Z", false},
	{"2021-W53", "", true},
	{"2026-41", "", true},
	{"2026-W00", "", true},
	{"2026-W54", "", true},
	{"0000-W01", "", true},
	{"xxxx-W01", "", true},
	{"2026-Wxx", "", true},
	{"+026-W01", "", true},
}

func TestDigestWeeks(t *testing.T) {
	for _, tc := range digestWeekCases {
		t.Run(tc.week, func(t *testing.T) {
			in := digestTestInput()
			in.Week = tc.week
			out, err := BuildDigest(in)
			if tc.wantErr {
				if !errors.Is(err, ErrUsage) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || !out.Start.Equal(digestTestTime(tc.start)) || !out.End.Equal(out.Start.AddDate(0, 0, 7)) {
				t.Fatalf("digest = %+v, error = %v", out, err)
			}
		})
	}
}

var digestDeliveryCases = []struct {
	name, closed string
	state        IssueState
	archived     bool
	want         int
}{
	{"start inclusive", "2026-10-05T03:00:00Z", IssueClosed, false, 1},
	{"end exclusive", "2026-10-12T03:00:00Z", IssueClosed, false, 0},
	{"previous local day", "2026-10-05T02:59:59Z", IssueClosed, false, 0},
	{"archived delivery", "2026-10-06T12:00:00Z", IssueClosed, true, 1},
	{"reopened", "2026-10-06T12:00:00Z", IssueOpen, false, 0},
	{"undated", "", IssueClosed, false, 0},
}

func TestDigestDeliveries(t *testing.T) {
	for _, tc := range digestDeliveryCases {
		t.Run(tc.name, func(t *testing.T) {
			in, item := digestTestInput(), digestTestItem(1)
			item.Issue.State, item.Archived = tc.state, tc.archived
			if tc.closed != "" {
				at := digestTestTime(tc.closed)
				item.Issue.ClosedAt = &at
			}
			in.Items = []Item{item}
			out, err := BuildDigest(in)
			if err != nil || len(out.Deliveries) != tc.want {
				t.Fatalf("deliveries = %v, error = %v", out.Deliveries, err)
			}
		})
	}
}

var digestBuildCases = []struct {
	name string
	edit func(*DigestInput)
}{
	{"generic", func(in *DigestInput) { in.Config = nil }},
	{"empty", func(in *DigestInput) { in.Config.Digest.Metrics = nil }},
	{"rich", digestTestRich},
}

func digestTestRich(in *DigestInput) {
	item := digestTestItem(1)
	at := digestTestTime("2026-10-08T12:00:00Z")
	item.Issue.State, item.Issue.ClosedAt = IssueClosed, &at
	item.Issue.Title = "\x1b[31mDelivery\n[title]"
	item.Issue.Milestone = &Milestone{ID: "milestone", Number: 3, Title: "Release", State: "OPEN", DueOn: &at}
	in.Items = []Item{item, digestTestItem(2)}
	in.Items[1].Issue.BlockedByCount = 1
	in.Config.Alerts = AlertRules{AlertBlocked: {}}
	in.Milestones = []DigestMilestone{{Repository: "sample/board", Milestone: *item.Issue.Milestone}}
}

func TestDigestContentAndPurity(t *testing.T) {
	for _, tc := range digestBuildCases {
		t.Run(tc.name, func(t *testing.T) {
			in := digestTestInput()
			tc.edit(&in)
			before, _ := json.Marshal(in)
			out, err := BuildDigest(in)
			after, _ := json.Marshal(in)
			if err != nil || !reflect.DeepEqual(before, after) || strings.Contains(out.Text, "\x1b") {
				t.Fatalf("mutation or error: %v", err)
			}
			digestCheckSections(t, out.Text)
			if tc.name == "rich" && (len(out.Milestones) != 1 || len(out.Risks) != 1) {
				t.Fatalf("rich digest = %+v", out)
			}
		})
	}
}

func digestCheckSections(t *testing.T, text string) {
	t.Helper()
	for _, section := range []string{"Entregas", "Marcos", "Riscos observados", "Métricas"} {
		if !strings.Contains(text, section) {
			t.Errorf("missing %s in %s", section, text)
		}
	}
}

func TestDigestMissingNow(t *testing.T) {
	for _, in := range []DigestInput{{}, {Config: digestTestConfig()}} {
		if _, err := BuildDigest(in); !errors.Is(err, ErrUsage) {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestDigestMilestoneWindow(t *testing.T) {
	for _, tc := range digestDeliveryCases {
		t.Run(tc.name, func(t *testing.T) {
			in := digestTestInput()
			var due *time.Time
			if tc.closed != "" {
				at := digestTestTime(tc.closed)
				due = &at
			}
			in.Milestones = []DigestMilestone{{Milestone: Milestone{Number: 1, DueOn: due}}}
			out, err := BuildDigest(in)
			want := 0
			loc := LocationOf(in.Config)
			if due != nil && digestInWeek(CalendarDate(*due), Today(out.Start, loc), Today(out.End, loc)) {
				want = 1
			}
			if err != nil || len(out.Milestones) != want {
				t.Fatalf("milestones = %v, error = %v", out.Milestones, err)
			}
		})
	}
}
