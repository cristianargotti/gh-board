package plan

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// JournalEntry is one line of journal/<plan id>.jsonl. Created carries the
// node id a step created, so a resumed plan keeps using it; Note explains
// a skip.
type JournalEntry struct {
	PlanID    string           `json:"plan_id"`
	Step      int              `json:"step"`
	Operation domain.Operation `json:"operation"`
	Target    domain.Target    `json:"target"`
	Status    string           `json:"status"`
	Error     string           `json:"error,omitempty"`
	Created   string           `json:"created,omitempty"`
	Note      string           `json:"note,omitempty"`
	At        time.Time        `json:"at"`
}

// Journal statuses.
const (
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// Journal appends and reads per-plan journal files.
type Journal struct {
	dir string
}

// NewJournal returns a journal whose files live at <dir>/journal.
func NewJournal(dir string) *Journal {
	return &Journal{dir: dir}
}

// Dir returns the state directory the journal lives under.
func (j *Journal) Dir() string { return j.dir }

// journalPath is the file of one plan.
func journalPath(dir, planID string) string {
	return filepath.Join(dir, JournalDir, planID+".jsonl")
}

// Append writes one entry for the plan.
func (j *Journal) Append(e JournalEntry) error {
	if !validID(e.PlanID) {
		return fmt.Errorf("journal: plan id %q is not valid: %w", e.PlanID, domain.ErrUsage)
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("journal: encode: %w", err)
	}
	return audit.AppendLine(journalPath(j.dir, e.PlanID), line)
}

// Read returns the entries of a plan in order. A plan without a journal
// has no entries; a line that does not decode is skipped.
func (j *Journal) Read(planID string) ([]JournalEntry, error) {
	if !validID(planID) {
		return nil, fmt.Errorf("journal: plan id %q is not valid: %w", planID, domain.ErrUsage)
	}
	lines, err := audit.ReadLines(journalPath(j.dir, planID))
	if err != nil {
		return nil, err
	}
	entries := make([]JournalEntry, 0, len(lines))
	for _, line := range lines {
		var e JournalEntry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// DoneSteps returns the steps a journal reports as done or skipped, by
// index. A failed line after a done line does not undo it.
func DoneSteps(entries []JournalEntry) map[int]JournalEntry {
	done := make(map[int]JournalEntry)
	for _, e := range entries {
		if e.Status == StatusDone || e.Status == StatusSkipped {
			done[e.Step] = e
		}
	}
	return done
}
