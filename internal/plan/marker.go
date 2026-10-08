package plan

import (
	"fmt"
	"strings"
	"time"
)

// Marker is the invisible HTML comment a plan step leaves in a comment
// body, so a resumed plan recognizes the comment it already posted.
func Marker(planID string, step int) string {
	return fmt.Sprintf("<!-- gh-board plan=%s step=%d -->", planID, step)
}

// WeekMarker tags a status update with its ISO week; digest post checks
// it together with the creation date.
func WeekMarker(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("<!-- gh-board digest=%04d-W%02d -->", year, week)
}

// SameISOWeek reports whether both instants fall in the same ISO week.
func SameISOWeek(a, b time.Time) bool {
	ay, aw := a.ISOWeek()
	by, bw := b.ISOWeek()
	return ay == by && aw == bw
}

func markedBody(body, marker string) string {
	if strings.Contains(body, marker) {
		return body
	}
	return strings.TrimRight(body, "\n") + "\n\n" + marker
}
