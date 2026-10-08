package domain_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var locationCases = []struct {
	name string
	cfg  *domain.Config
	want string
}{
	{"generic", nil, "UTC"},
	{"unset", &domain.Config{}, "UTC"},
	{"unknown zone falls back to UTC", &domain.Config{Timezone: "Mars/Olympus"}, "UTC"},
	{"configured", fixtureConfig(), fxZone},
}

func TestLocationOf(t *testing.T) {
	for _, tc := range locationCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.LocationOf(tc.cfg); got.String() != tc.want {
				t.Fatalf("LocationOf = %s, want %s", got, tc.want)
			}
		})
	}
}

var parseDateCases = []struct {
	in   string
	want time.Time
	ok   bool
}{
	{"2026-10-08", time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), true},
	{" 2026-10-08 ", time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), true},
	{"2026-10-09T01:00:00Z", time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), true},
	{"", time.Time{}, false},
	{"08/10/2026", time.Time{}, false},
	{"2026-13-01", time.Time{}, false},
	{"yesterday", time.Time{}, false},
}

func TestParseDate(t *testing.T) {
	for _, tc := range parseDateCases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := domain.ParseDate(tc.in, fxLoc)
			if ok != tc.ok || !got.Equal(tc.want) {
				t.Fatalf("ParseDate = %s, %v, want %s, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestDatesCapabilityFieldName(t *testing.T) {
	var absent *domain.DatesCapability
	if absent.FieldName(domain.DateStart) != "" || absent.FieldName(domain.DateTarget) != "" {
		t.Fatal("an absent capability names no field")
	}
	dates := fixtureConfig().Capabilities.Dates
	if dates.FieldName(domain.DateStart) != fxStart || dates.FieldName(domain.DateTarget) != fxTarget {
		t.Fatalf("FieldName = %q, %q", dates.FieldName(domain.DateStart), dates.FieldName(domain.DateTarget))
	}
}

var projectDates = &domain.DatesCapability{Start: fxStart, Target: fxTarget, Source: domain.DateSourceProject}

var itemDateCases = []struct {
	name  string
	item  domain.Item
	dates *domain.DatesCapability
	kind  domain.DateKind
	text  string
	ok    bool
}{
	{"issue field target", newItem(1, "a", withIssueField(fxTarget, "2026-10-20")), fixtureConfig().Capabilities.Dates, domain.DateTarget, "2026-10-20", true},
	{"issue field start", newItem(1, "a", withIssueField(fxStart, "2026-10-01")), fixtureConfig().Capabilities.Dates, domain.DateStart, "2026-10-01", true},
	{"issue field missing", newItem(1, "a"), fixtureConfig().Capabilities.Dates, domain.DateTarget, "", false},
	{"project field", newItem(1, "a", withField(fxTarget, "2026-10-20")), projectDates, domain.DateTarget, "2026-10-20", true},
	{"project field ignores issue fields", newItem(1, "a", withIssueField(fxTarget, "2026-10-20")), projectDates, domain.DateTarget, "", false},
	{"absent capability", newItem(1, "a", withIssueField(fxTarget, "2026-10-20")), nil, domain.DateTarget, "", false},
	{"unparseable value", newItem(1, "a", withIssueField(fxTarget, "soon")), fixtureConfig().Capabilities.Dates, domain.DateTarget, "soon", false},
}

func TestItemDate(t *testing.T) {
	for _, tc := range itemDateCases {
		t.Run(tc.name, func(t *testing.T) {
			if text := domain.ItemDateText(tc.item, tc.dates, tc.kind); text != tc.text {
				t.Fatalf("ItemDateText = %q, want %q", text, tc.text)
			}
			got, ok := domain.ItemDate(tc.item, tc.dates, tc.kind, fxLoc)
			if ok != tc.ok {
				t.Fatalf("ItemDate = %s, %v, want ok %v", got, ok, tc.ok)
			}
			if ok && got.Format(domain.DateLayout) != tc.text {
				t.Fatalf("ItemDate = %s, want %s", got, tc.text)
			}
		})
	}
}

func TestFormatDay(t *testing.T) {
	if got := domain.FormatDay(fxNow, fxLoc); got != "08/10/2026" {
		t.Fatalf("FormatDay = %q", got)
	}
	utc := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	if got := domain.FormatDay(utc, fxLoc); got != "08/10/2026" {
		t.Fatalf("FormatDay must render in the board timezone, got %q", got)
	}
}

func TestCalendarDates(t *testing.T) {
	due := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if got := domain.Today(fxNow, fxLoc); !got.Equal(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Today = %s", got)
	}
	// 23:30 in Sao Paulo is already the next day in UTC; the board date wins.
	late := time.Date(2026, 10, 8, 23, 30, 0, 0, fxLoc)
	if got := domain.Today(late, fxLoc); got.Day() != 8 {
		t.Fatalf("Today near midnight = %s", got)
	}
	if got := domain.CalendarDate(due.Add(7 * time.Hour)); !got.Equal(due) {
		t.Fatalf("CalendarDate = %s", got)
	}
	if domain.DateText(due) != "2026-10-09" || domain.DayText(due) != "09/10/2026" {
		t.Fatalf("DateText = %q, DayText = %q", domain.DateText(due), domain.DayText(due))
	}
	if got := domain.DaysFromToday(due, fxNow, fxLoc); got != 1 {
		t.Fatalf("DaysFromToday = %d", got)
	}
	if got := domain.DaysFromToday(due.AddDate(0, 0, -3), fxNow, fxLoc); got != -2 {
		t.Fatalf("DaysFromToday past = %d", got)
	}
}

func TestIterationSummary(t *testing.T) {
	it := domain.Iteration{Title: "Sprint 5", Start: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), Duration: 14}
	sum := domain.IterationSummary(it, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC))
	if sum.Start != "2026-09-25" || sum.End != "2026-10-08" || sum.DaysLeft != 0 || sum.Title != "Sprint 5" {
		t.Fatalf("IterationSummary = %+v", sum)
	}
}
