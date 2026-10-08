package domain_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func localDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, fxLoc)
}

var businessDayCases = []struct {
	t    time.Time
	want bool
}{
	{fxNow, true},                                          // Thursday
	{fxNow.AddDate(0, 0, 1), true},                         // Friday
	{fxNow.AddDate(0, 0, 2), false},                        // Saturday
	{fxNow.AddDate(0, 0, 3), false},                        // Sunday
	{fxNow.AddDate(0, 0, 4), true},                         // Monday
	{time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC), false}, // Saturday 22:00 in Sao Paulo
}

func TestIsBusinessDay(t *testing.T) {
	for _, tc := range businessDayCases {
		if got := domain.IsBusinessDay(tc.t, fxLoc); got != tc.want {
			t.Errorf("IsBusinessDay(%s) = %v, want %v", tc.t, got, tc.want)
		}
	}
}

func TestDateOfAndSameDay(t *testing.T) {
	lateUTC := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC) // 22:00 of October 8 in Sao Paulo
	if got := domain.DateOf(lateUTC, fxLoc); !got.Equal(localDay(2026, 10, 8)) || got.Location() != fxLoc {
		t.Fatalf("DateOf = %s", got)
	}
	if !domain.SameDay(lateUTC, fxNow, fxLoc) {
		t.Fatal("22:00 local and noon local share the day")
	}
	if domain.SameDay(lateUTC, fxNow, time.UTC) {
		t.Fatal("in UTC the instants fall on different days")
	}
	if domain.SameDay(fxNow, fxNow.AddDate(0, 0, 1), fxLoc) {
		t.Fatal("different days must not match")
	}
}

var newYork = mustLoc("America/New_York")

var daysBetweenCases = []struct {
	name string
	from time.Time
	to   time.Time
	loc  *time.Location
	want int
}{
	{"forward", fxNow, fxNow.AddDate(0, 0, 2), fxLoc, 2},
	{"backward", fxNow.AddDate(0, 0, 2), fxNow, fxLoc, -2},
	{"same day", fxNow, fxNow.Add(5 * time.Hour), fxLoc, 0},
	{"midnight edge", time.Date(2026, 10, 8, 23, 59, 0, 0, fxLoc), time.Date(2026, 10, 9, 0, 1, 0, 0, fxLoc), fxLoc, 1},
	{"across daylight saving", time.Date(2026, 3, 7, 12, 0, 0, 0, newYork), time.Date(2026, 3, 9, 12, 0, 0, 0, newYork), newYork, 2},
}

func TestDaysBetween(t *testing.T) {
	for _, tc := range daysBetweenCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.DaysBetween(tc.from, tc.to, tc.loc); got != tc.want {
				t.Fatalf("DaysBetween = %d, want %d", got, tc.want)
			}
		})
	}
}

var businessDaysBetweenCases = []struct {
	name string
	from time.Time
	to   time.Time
	want int
}{
	{"thursday to friday", fxNow, fxNow.AddDate(0, 0, 1), 1},
	{"thursday to saturday", fxNow, fxNow.AddDate(0, 0, 2), 1},
	{"thursday to monday", fxNow, fxNow.AddDate(0, 0, 4), 2},
	{"thursday to tuesday", fxNow, fxNow.AddDate(0, 0, 5), 3},
	{"friday to monday", fxNow.AddDate(0, 0, 1), fxNow.AddDate(0, 0, 4), 1},
	{"same day", fxNow, fxNow.Add(time.Hour), 0},
	{"backwards", fxNow.AddDate(0, 0, 4), fxNow, 0},
	{"two full weeks", fxNow, fxNow.AddDate(0, 0, 14), 10},
}

func TestBusinessDaysBetween(t *testing.T) {
	for _, tc := range businessDaysBetweenCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.BusinessDaysBetween(tc.from, tc.to, fxLoc); got != tc.want {
				t.Fatalf("BusinessDaysBetween = %d, want %d", got, tc.want)
			}
		})
	}
}

var addBusinessDaysCases = []struct {
	name string
	from time.Time
	n    int
	want time.Time
}{
	{"thursday plus one", fxNow, 1, fxNow.AddDate(0, 0, 1)},
	{"thursday plus two skips the weekend", fxNow, 2, fxNow.AddDate(0, 0, 4)},
	{"friday plus one", fxNow.AddDate(0, 0, 1), 1, fxNow.AddDate(0, 0, 4)},
	{"saturday plus one", fxNow.AddDate(0, 0, 2), 1, fxNow.AddDate(0, 0, 4)},
	{"zero keeps the instant", fxNow, 0, fxNow},
	{"negative keeps the instant", fxNow, -3, fxNow},
	{"from utc keeps the clock in the location", time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC), 1, fxNow.AddDate(0, 0, 1)},
}

func TestAddBusinessDays(t *testing.T) {
	for _, tc := range addBusinessDaysCases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.AddBusinessDays(tc.from, tc.n, fxLoc)
			if !got.Equal(tc.want) || got.Location() != fxLoc {
				t.Fatalf("AddBusinessDays = %s, want %s", got, tc.want)
			}
		})
	}
}

var weekStartCases = []struct {
	name string
	t    time.Time
	want time.Time
}{
	{"thursday", fxNow, localDay(2026, 10, 5)},
	{"monday", localDay(2026, 10, 5).Add(9 * time.Hour), localDay(2026, 10, 5)},
	{"sunday", localDay(2026, 10, 11).Add(23 * time.Hour), localDay(2026, 10, 5)},
	{"next monday", localDay(2026, 10, 12), localDay(2026, 10, 12)},
	{"utc instant late sunday", time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC), localDay(2026, 10, 5)},
}

func TestWeekStart(t *testing.T) {
	for _, tc := range weekStartCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.WeekStart(tc.t, fxLoc); !got.Equal(tc.want) {
				t.Fatalf("WeekStart = %s, want %s", got, tc.want)
			}
		})
	}
}
