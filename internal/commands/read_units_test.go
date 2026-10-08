package commands

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var readSinceCases = []struct {
	in   string
	want time.Time
	ok   bool
}{
	{"", readFixtureNow.AddDate(0, 0, -7), true},
	{"7d", readFixtureNow.AddDate(0, 0, -7), true},
	{"2w", readFixtureNow.AddDate(0, 0, -14), true},
	{"24h", readFixtureNow.Add(-24 * time.Hour), true},
	{"2026-10-01T10:00:00Z", time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), true},
	{"2026-10-01", time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local), true},
	{"yesterday", time.Time{}, false},
	{"-5d", time.Time{}, false},
	{"d", time.Time{}, false},
}

func TestReadParseSince(t *testing.T) {
	for _, tc := range readSinceCases {
		got, err := readParseSince(tc.in, readFixtureNow)
		if (err == nil) != tc.ok {
			t.Errorf("%q: err = %v", tc.in, err)
			continue
		}
		if tc.ok && !got.Equal(tc.want) {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

var readIterationCases = []struct {
	name   string
	now    time.Time
	which  string
	title  string
	reason string
}{
	{"current", readFixtureNow, readSprintCurrent, "Sprint 5", ""},
	{"next", readFixtureNow, readSprintNext, "Sprint 6", ""},
	{"next without current", readDate("2026-09-01"), readSprintNext, "Sprint 4", ""},
	{"no current", readDate("2027-06-01"), readSprintCurrent, "", "no sprint in progress"},
	{"no next", readDate("2027-06-01"), readSprintNext, "", "no sprint scheduled after the current one"},
	{"title", readFixtureNow, "sprint 4", "Sprint 4", ""},
	{"unknown title", readFixtureNow, "Sprint 9", "", "not found"},
}

func TestReadResolveIteration(t *testing.T) {
	cfg := readFixtureConfig(t).Config
	project := readFixtureProject()
	for _, tc := range readIterationCases {
		t.Run(tc.name, func(t *testing.T) {
			_, it, reason := readResolveIteration(cfg, project, tc.now, tc.which)
			if it.Title != tc.title || !strings.Contains(reason, tc.reason) || (reason == "") != (tc.reason == "") {
				t.Fatalf("got %q, %q", it.Title, reason)
			}
		})
	}
	project.Fields = project.Fields[:2]
	if _, _, reason := readResolveIteration(cfg, project, readFixtureNow, readSprintCurrent); !strings.Contains(reason, "not found on the board") {
		t.Fatalf("reason = %q", reason)
	}
}

func TestReadSprintNotes(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	deps.Clock = readFakeClock{readDate("2027-06-01")}
	for _, args := range [][]string{{"sprint", "current"}, {"sprint", "next"}} {
		out.Reset()
		if err := readRun(t, deps, args...); err != nil || !strings.Contains(out.String(), "no sprint") {
			t.Fatalf("%v: %v %s", args, err, out.String())
		}
	}
}

func TestReadIterationState(t *testing.T) {
	it := domain.Iteration{Title: "S", Start: readDate("2026-10-05"), Duration: 14}
	if readIterationState(it, readFixtureNow) != readSprintCurrent {
		t.Fatal("current")
	}
	if readIterationState(it, readDate("2026-09-01")) != "upcoming" {
		t.Fatal("upcoming")
	}
	if readIterationState(it, readDate("2026-11-01")) != "completed" {
		t.Fatal("past end")
	}
	it.Completed = true
	if readIterationState(it, readFixtureNow) != "completed" {
		t.Fatal("completed flag")
	}
}

func TestReadSchemaRefresh(t *testing.T) {
	plain := readNewFakeReader()
	deps, out := readTestDeps(t, plain)
	if err := readRun(t, deps, "schema", "--refresh"); err != nil || !strings.Contains(out.String(), "refresh not supported") {
		t.Fatalf("plain: %v %s", err, out.String())
	}
	refresher := readFakeRefresher{readNewFakeReader()}
	deps, out = readTestDeps(t, refresher)
	if err := readRun(t, deps, "--json", "schema", "--refresh"); err != nil {
		t.Fatal(err)
	}
	var payload readSchemaPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || !payload.Refreshed || payload.Note != "" {
		t.Fatalf("payload = %+v, err = %v", payload, err)
	}
	if strings.Join(refresher.calls, ",") != "RefreshProject" {
		t.Fatalf("calls = %v", refresher.calls)
	}
	refresher.fail["RefreshProject"] = errors.New("boom")
	if got := readRunCode(t, deps, "schema", "--refresh"); got != domain.ExitAPI {
		t.Fatalf("exit = %v", got)
	}
}

func TestReadDigestTimelines(t *testing.T) {
	base := readNewFakeReader()
	timeline := domain.IssueTimeline{
		Comments: []domain.Comment{{ID: "IC_1", Author: "ana", Body: "Vamos ensinar.", CreatedAt: readFixtureNow.Add(-20 * time.Hour)}},
		Complete: true,
	}
	fake := readFakeTimelines{readFakeReader: base, timelines: map[string]domain.IssueTimeline{"I_kwDOAbc008": timeline}}
	deps, out := readTestDeps(t, fake)
	if err := readRun(t, deps, "digest", "print"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "first_response: 100,0% (1/1)") {
		t.Fatalf("output = %s", out.String())
	}
	deps, _ = readTestDeps(t, readFakeTimelines{readFakeReader: base, err: errors.New("boom")})
	if got := readRunCode(t, deps, "digest", "print"); got != domain.ExitAPI {
		t.Fatalf("exit = %v", got)
	}
}

func TestReadDigestJSONCarriesText(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	if err := readRun(t, deps, "--json", "digest", "print"); err != nil {
		t.Fatal(err)
	}
	var result domain.DigestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Text, "Resumo da semana 2026-W41") || result.Milestones == nil {
		t.Fatalf("result = %+v", result)
	}
}

func TestReadWindowMonths(t *testing.T) {
	w := readWindowOf(readFixtureNow, time.UTC, 3)
	if strings.Join(w.labels, ",") != "2026-10,2026-11,2026-12" {
		t.Fatalf("labels = %v", w.labels)
	}
	got := w.months(readDate("2026-11-20"), readDate("2026-12-01"))
	if strings.Join(got, ",") != "2026-11" {
		t.Fatalf("months = %v", got)
	}
	if len(w.months(readDate("2027-01-01"), readDate("2027-02-01"))) != 0 {
		t.Fatal("outside the window")
	}
}

func TestReadSortMilestones(t *testing.T) {
	rows := []readRoadmapMilestone{{Title: "b"}, {Title: "z", DueOn: "2026-12-01"}, {Title: "a", DueOn: "2026-11-01"}, {Title: "a"}}
	readSortMilestones(rows)
	got := []string{}
	for _, r := range rows {
		got = append(got, r.Title+":"+r.DueOn)
	}
	if strings.Join(got, ",") != "a:2026-11-01,z:2026-12-01,a:,b:" {
		t.Fatalf("order = %v", got)
	}
}

func TestReadRoadmapProjectDates(t *testing.T) {
	fake := readNewFakeReader()
	deps, out := readTestDeps(t, fake)
	deps.Config.Config.Capabilities.Dates = &domain.DatesCapability{Start: "Start date", Target: "Target date", Source: domain.DateSourceProject}
	fake.items[0].Values["Target date"] = domain.FieldValue{Field: "Target date", Value: "2026-10-30"}
	if err := readRun(t, deps, "roadmap", "--months", "1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "2026-10-30  2/4 (50%)  ====") {
		t.Fatalf("output = %s", out.String())
	}
}
