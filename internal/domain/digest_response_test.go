package domain

import (
	"strings"
	"testing"
	"time"
)

var digestResponseCases = []struct {
	name, created, responded string
	urgent                   bool
	matched                  bool
}{
	{"same day", "2026-10-05T12:00:00Z", "2026-10-05T13:00:00Z", false, true},
	{"deadline day", "2026-10-05T12:00:00Z", "2026-10-08T02:59:59Z", false, true},
	{"after deadline", "2026-10-05T12:00:00Z", "2026-10-08T03:00:00Z", false, false},
	{"weekend excluded", "2026-10-09T12:00:00Z", "2026-10-13T23:00:00Z", false, true},
	{"urgent same day", "2026-10-05T12:00:00Z", "2026-10-06T02:59:59Z", true, true},
	{"urgent next day", "2026-10-05T12:00:00Z", "2026-10-06T03:00:00Z", true, false},
}

func TestDigestFirstResponseSLA(t *testing.T) {
	for _, tc := range digestResponseCases {
		t.Run(tc.name, func(t *testing.T) {
			in, it := digestTestInput(), digestTestItem(1)
			in.Now = digestTestTime("2026-10-16T15:00:00Z")
			in.Config.Digest.Metrics = []string{"first_response"}
			it.Issue.CreatedAt = digestTestTime(tc.created)
			if tc.urgent {
				it.Issue.Labels = append(it.Issue.Labels, Label{Name: "rush"})
			}
			in.Items = []Item{it}
			in.Timelines[it.Issue.NodeID] = IssueTimeline{Complete: true, Comments: []Comment{
				{ID: "later", Author: "member", CreatedAt: in.Now},
				{ID: "first", Author: "MEMBER", CreatedAt: digestTestTime(tc.responded)},
				{ID: "outsider", Author: "someone", CreatedAt: it.Issue.CreatedAt},
			}}
			out, err := BuildDigest(in)
			m := out.Metrics[0]
			if err != nil || !m.Available || m.Evidence[0].ID != "first" || m.Evidence[0].Matched != tc.matched {
				t.Fatalf("metric = %+v, error = %v", m, err)
			}
		})
	}
}

var digestTimelineCases = []struct {
	name, reason string
	edit         func(*DigestInput, *IssueTimeline)
	want         int
}{
	{"empty expired", "", func(_ *DigestInput, _ *IssueTimeline) {}, 0},
	{"pending", "andamento", func(in *DigestInput, _ *IssueTimeline) { in.Now = digestTestTime("2026-10-06T12:00:00Z") }, 0},
	{"partial", "página", func(_ *DigestInput, tl *IssueTimeline) { tl.Complete, tl.Reason = false, "página ausente" }, 0},
	{"bad comment", "autor ou data", func(_ *DigestInput, tl *IssueTimeline) { tl.Comments = []Comment{{Author: "member"}} }, 0},
	{"bad decision", "autor ou data", func(_ *DigestInput, tl *IssueTimeline) {
		tl.Decisions = []DecisionEvent{{Field: "Decision", Value: "Ensinar"}}
	}, 0},
	{"decision", "", func(_ *DigestInput, tl *IssueTimeline) { tl.Decisions = digestTestDecisions() }, 1},
	{"unrelated decision", "", func(_ *DigestInput, tl *IssueTimeline) {
		tl.Decisions = []DecisionEvent{{Field: "Other", Value: "Ensinar"}}
	}, 0},
	{"empty decision", "", func(_ *DigestInput, tl *IssueTimeline) { tl.Decisions = []DecisionEvent{{Field: "Decision"}} }, 0},
	{"default SLA", "", func(in *DigestInput, tl *IssueTimeline) {
		in.Config.Capabilities.Triage.SLABusinessDays = 0
		tl.Decisions = digestTestDecisions()
	}, 1},
	{"out of time", "", digestTestOutsideEvents, 0},
}

func digestTestDecisions() []DecisionEvent {
	return []DecisionEvent{{ID: "decision", Field: "Decision", Value: "Ensinar", Actor: "member", CreatedAt: digestTestTime("2026-10-06T12:00:00Z")}}
}

func digestTestOutsideEvents(_ *DigestInput, tl *IssueTimeline) {
	tl.Comments = []Comment{
		{Author: "member", CreatedAt: digestTestTime("2026-10-04T12:00:00Z")},
		{Author: "member", CreatedAt: digestTestTime("2026-10-15T12:00:00Z")},
	}
}

func TestDigestTimelineEvidence(t *testing.T) {
	for _, tc := range digestTimelineCases {
		t.Run(tc.name, func(t *testing.T) {
			in, it := digestTestInput(), digestTestItem(1)
			in.Config.Digest.Metrics = []string{"first_response"}
			tl := IssueTimeline{Complete: true}
			tc.edit(&in, &tl)
			in.Items, in.Timelines[it.Issue.NodeID] = []Item{it}, tl
			out, err := BuildDigest(in)
			m := out.Metrics[0]
			if err != nil || m.Available != (tc.reason == "") || m.Numerator != tc.want || !strings.Contains(m.Reason, tc.reason) {
				t.Fatalf("metric = %+v, error = %v", m, err)
			}
		})
	}
}

func TestDigestResponseCohort(t *testing.T) {
	for _, date := range []string{"2026-10-04T12:00:00Z", "2026-10-12T03:00:00Z", "2026-10-11T12:00:00Z"} {
		t.Run(date, func(t *testing.T) {
			in, it := digestTestInput(), digestTestItem(1)
			it.Issue.CreatedAt = digestTestTime(date)
			in.Items, in.Now = []Item{it}, digestTestTime("2026-10-09T12:00:00Z")
			in.Config.Digest.Metrics = []string{"first_response"}
			out, err := BuildDigest(in)
			if err != nil || out.Metrics[0].Denominator != 0 {
				t.Fatalf("metric = %+v, error = %v", out.Metrics, err)
			}
		})
	}
}

func TestDigestFirstEventTie(t *testing.T) {
	for _, events := range [][]MetricEvidence{nil, {{Actor: "member", At: time.Time{}}}} {
		if _, found := digestFirstMemberEvent(events, []string{"member"}, digestTestTime("2026-10-01T00:00:00Z"), digestTestTime("2026-10-08T00:00:00Z")); found {
			t.Fatal("invalid event accepted")
		}
	}
}
