package domain

import (
	"math"
	"time"
)

// IsBusinessDay reports whether the instant falls on Monday to Friday in
// the location.
func IsBusinessDay(t time.Time, loc *time.Location) bool {
	wd := t.In(loc).Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// DateOf truncates an instant to its local date at midnight.
func DateOf(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
}

// SameDay reports whether two instants share a local date.
func SameDay(a, b time.Time, loc *time.Location) bool {
	return DateOf(a, loc).Equal(DateOf(b, loc))
}

// DaysBetween counts the calendar days from the date of from to the date
// of to; negative when to is earlier. Rounding absorbs daylight changes.
func DaysBetween(from, to time.Time, loc *time.Location) int {
	a, b := DateOf(from, loc), DateOf(to, loc)
	return int(math.Round(b.Sub(a).Hours() / 24))
}

// BusinessDaysBetween counts the business days after the date of from up
// to and including the date of to; zero when to is not after from.
func BusinessDaysBetween(from, to time.Time, loc *time.Location) int {
	days := DaysBetween(from, to, loc)
	if days <= 0 {
		return 0
	}
	start := DateOf(from, loc)
	count := 0
	for i := 1; i <= days; i++ {
		if IsBusinessDay(start.AddDate(0, 0, i), loc) {
			count++
		}
	}
	return count
}

// AddBusinessDays moves the instant forward by n business days, keeping
// the clock time. A non-positive n returns the instant in the location.
func AddBusinessDays(t time.Time, n int, loc *time.Location) time.Time {
	out := t.In(loc)
	for n > 0 {
		out = out.AddDate(0, 0, 1)
		if IsBusinessDay(out, loc) {
			n--
		}
	}
	return out
}

// WeekStart returns Monday at midnight of the week that contains t.
func WeekStart(t time.Time, loc *time.Location) time.Time {
	day := DateOf(t, loc)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}
