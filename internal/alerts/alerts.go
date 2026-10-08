// Package alerts diffs alert states, writes alerts.json, notifies through
// the native notifier of each operating system and installs the native
// scheduler for watch (section 9). Every executable runs through an
// injectable Runner, so tests never launch a notifier or register a
// scheduler, and the user directories arrive through the options, so this
// package reads no environment.
package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Defaults of watch.
const (
	// StateFileName is the file the Claude Code mod reads.
	StateFileName = "alerts.json"
	// LockFileName is the single-instance lock of watch, under the state
	// directory.
	LockFileName = "watch.lock"
	// DefaultInterval is the scheduler interval of watch --install.
	DefaultInterval = 10 * time.Minute
	// MaxToastsPerHour caps notifications; the rest stay in alerts.json.
	MaxToastsPerHour = 5
)

// Diff is the change between two alert states.
type Diff struct {
	New       []domain.Alert
	Escalated []domain.Alert
	Resolved  []domain.Alert
	Unchanged int
}

// Notifiable lists the alerts watch notifies: the new ones, then the
// escalated ones, each in the order of the current state.
func (d Diff) Notifiable() []domain.Alert {
	out := make([]domain.Alert, 0, len(d.New)+len(d.Escalated))
	out = append(out, d.New...)
	return append(out, d.Escalated...)
}

// Evaluate diffs the previous state against the current one: alerts whose
// fingerprint is new, alerts whose severity rose and alerts that vanished.
// FirstSeen is carried over from the previous state for the alerts it
// knew. A previous state of another project knows none of the current
// alerts, so every one of them is new and none is resolved. A state that
// repeats a fingerprint or carries an alert without a rule is an error.
func Evaluate(previous, current domain.AlertState) (Diff, error) {
	known, err := index(previous)
	if err != nil {
		return Diff{}, fmt.Errorf("previous alert state: %w", err)
	}
	now, err := index(current)
	if err != nil {
		return Diff{}, fmt.Errorf("current alert state: %w", err)
	}
	if !sameProject(previous.Project, current.Project) {
		previous = domain.AlertState{}
		known = nil
	}
	var diff Diff
	for _, a := range current.Alerts {
		old, seen := known[a.Fingerprint()]
		switch {
		case !seen:
			diff.New = append(diff.New, a)
		case a.Severity.Rank() > old.Severity.Rank():
			diff.Escalated = append(diff.Escalated, carryFirstSeen(a, old))
		default:
			diff.Unchanged++
		}
	}
	for _, a := range previous.Alerts {
		if _, still := now[a.Fingerprint()]; !still {
			diff.Resolved = append(diff.Resolved, a)
		}
	}
	return diff, nil
}

// index maps a state by fingerprint, refusing duplicates and alerts without
// a rule: both would make the diff lie.
func index(state domain.AlertState) (map[string]domain.Alert, error) {
	out := make(map[string]domain.Alert, len(state.Alerts))
	for i, a := range state.Alerts {
		if a.RuleID == "" {
			return nil, fmt.Errorf("alert %d has no rule id", i)
		}
		fp := a.Fingerprint()
		if _, dup := out[fp]; dup {
			return nil, fmt.Errorf("alert %d repeats %s", i, fp)
		}
		out[fp] = a
	}
	return out, nil
}

// sameProject treats an unset reference as compatible with any project,
// because a state written before the project was known still counts.
func sameProject(a, b domain.ProjectRef) bool {
	if a.IsZero() || b.IsZero() {
		return true
	}
	return strings.EqualFold(a.Owner, b.Owner) && a.Number == b.Number
}

// carryFirstSeen keeps the earliest known instant of an alert.
func carryFirstSeen(a, old domain.Alert) domain.Alert {
	if !old.FirstSeen.IsZero() && (a.FirstSeen.IsZero() || old.FirstSeen.Before(a.FirstSeen)) {
		a.FirstSeen = old.FirstSeen
	}
	return a
}
