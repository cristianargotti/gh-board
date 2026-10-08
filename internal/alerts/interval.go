package alerts

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Interval bounds of the scheduler: launchd, systemd, cron and Task
// Scheduler all take whole minutes, and a day is the longest gap that still
// makes the alerts timely.
const (
	MinInterval = time.Minute
	MaxInterval = 24 * time.Hour
)

// ParseInterval reads the --interval flag: a Go duration such as 10m or
// 1h30m, or a bare number of minutes. Empty means DefaultInterval. The
// result is a whole number of minutes between MinInterval and MaxInterval.
func ParseInterval(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultInterval, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		minutes, convErr := strconv.Atoi(s)
		if convErr != nil {
			return 0, fmt.Errorf("interval %q: use a duration such as 10m or 1h: %w", s, domain.ErrUsage)
		}
		d = time.Duration(minutes) * time.Minute
	}
	if err := validateInterval(d); err != nil {
		return 0, err
	}
	return d, nil
}

// validateInterval applies the bounds every scheduler shares.
func validateInterval(d time.Duration) error {
	switch {
	case d < MinInterval || d > MaxInterval:
		return fmt.Errorf("interval %s: must be between %s and %s: %w", d, MinInterval, MaxInterval, domain.ErrUsage)
	case d%time.Minute != 0:
		return fmt.Errorf("interval %s: must be whole minutes: %w", d, domain.ErrUsage)
	}
	return nil
}

// minutes is the interval as the schedulers count it.
func minutes(d time.Duration) int { return int(d / time.Minute) }
