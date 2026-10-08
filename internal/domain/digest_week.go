package domain

import (
	"fmt"
	"strconv"
	"time"
)

func digestWeekStart(week string, now time.Time, loc *time.Location) (time.Time, error) {
	if week == "" {
		return WeekStart(now, loc), nil
	}
	if len(week) != 8 || week[4:6] != "-W" {
		return time.Time{}, fmt.Errorf("digest: expected ISO week YYYY-Www: %w", ErrUsage)
	}
	year, yearErr := strconv.Atoi(week[:4])
	number, weekErr := strconv.Atoi(week[6:])
	if yearErr != nil || weekErr != nil || year < 1 || number < 1 || number > 53 {
		return time.Time{}, fmt.Errorf("digest: invalid ISO week %q: %w", week, ErrUsage)
	}
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, loc)
	start := WeekStart(jan4, loc).AddDate(0, 0, (number-1)*7)
	y, w := start.ISOWeek()
	if y != year || w != number || fmt.Sprintf("%04d-W%02d", y, w) != week {
		return time.Time{}, fmt.Errorf("digest: nonexistent ISO week %q: %w", week, ErrUsage)
	}
	return start, nil
}
