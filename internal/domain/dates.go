package domain

import (
	"math"
	"strings"
	"time"
)

// DateLayout is the form of issue and project date values.
const DateLayout = "2006-01-02"

// DayLayout is how the team reads a date in messages.
const DayLayout = "02/01/2006"

// DateKind selects the start or the target date of an item.
type DateKind string

// Date kinds of the dates capability.
const (
	DateStart  DateKind = "start"
	DateTarget DateKind = "target"
)

// Calendar dates (iteration starts, milestone due dates, date field
// values) carry no time zone: GitHub stores and returns them as the day
// at midnight UTC. The kit keeps them as UTC midnight instants, formats
// them without conversion and compares them with the board-zone date of
// now (Today), so a date never shifts by a day west of Greenwich.

// LocationOf loads the board timezone; UTC when unset or unknown.
func LocationOf(cfg *Config) *time.Location {
	if cfg == nil || cfg.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// ParseDate reads a YYYY-MM-DD value as a calendar date, or an RFC 3339
// instant as the calendar date it falls on in loc.
func ParseDate(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.ParseInLocation(DateLayout, s, time.UTC); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return Today(t, loc), true
	}
	return time.Time{}, false
}

// Today returns the calendar date of the instant in loc.
func Today(now time.Time, loc *time.Location) time.Time {
	l := now.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

// CalendarDate normalizes a date-only value the API returned as an
// instant (midnight UTC) to the calendar date form.
func CalendarDate(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// DateText renders a calendar date as YYYY-MM-DD, without conversion.
func DateText(date time.Time) string {
	return date.UTC().Format(DateLayout)
}

// DayText renders a calendar date the way the team reads it, without
// conversion.
func DayText(date time.Time) string {
	return date.UTC().Format(DayLayout)
}

// DaysFromToday counts the days from the calendar date of now in loc to
// the calendar date; negative when the date is past.
func DaysFromToday(date, now time.Time, loc *time.Location) int {
	return int(math.Round(CalendarDate(date).Sub(Today(now, loc)).Hours() / 24))
}

// FieldName returns the field that holds the date kind, or "" when the
// capability is absent.
func (d *DatesCapability) FieldName(kind DateKind) string {
	if d == nil {
		return ""
	}
	if kind == DateStart {
		return d.Start
	}
	return d.Target
}

// ItemDateText returns the raw date value from the configured source:
// organization issue fields or project fields.
func ItemDateText(it Item, dates *DatesCapability, kind DateKind) string {
	name := dates.FieldName(kind)
	if name == "" {
		return ""
	}
	if dates.Source == DateSourceIssueFields {
		v, _ := it.IssueField(name)
		return v.Value
	}
	return it.Text(name)
}

// ItemDate parses the start or target date of an item as a calendar date.
func ItemDate(it Item, dates *DatesCapability, kind DateKind, loc *time.Location) (time.Time, bool) {
	return ParseDate(ItemDateText(it, dates, kind), loc)
}

// FormatDay renders an instant as the day it falls on in loc, the way
// the team reads it. Calendar dates use DayText instead.
func FormatDay(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(DayLayout)
}
