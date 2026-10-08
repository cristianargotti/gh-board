package audit

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

// Path returns the audit log path under the state directory.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// Append adds one line to <dir>/audit.jsonl, creating the file with
// owner-only permissions. A zero At is stamped with the current time so
// every line carries an instant.
func Append(dir string, e Entry) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encode audit entry: %w", err)
	}
	return AppendLine(Path(dir), line)
}

// Query returns the entries at or after since, oldest first. Lines that
// do not decode (a partial write) are skipped: the log is evidence and a
// damaged line must not hide the rest.
func Query(dir string, since time.Time) ([]Entry, error) {
	entries, err := readAll(dir)
	if err != nil {
		return nil, err
	}
	kept := entries[:0]
	for _, e := range entries {
		if !e.At.Before(since) {
			kept = append(kept, e)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].At.Before(kept[j].At) })
	return kept, nil
}

// Prune drops entries older than before, rewrites the log atomically and
// returns how many lines were removed. Lines that do not decode count as
// removed.
func Prune(dir string, before time.Time) (int, error) {
	lines, err := ReadLines(Path(dir))
	if err != nil || len(lines) == 0 {
		return 0, err
	}
	var kept []byte
	removed := 0
	for _, line := range lines {
		var e Entry
		if json.Unmarshal(line, &e) != nil || e.At.Before(before) {
			removed++
			continue
		}
		kept = append(kept, line...)
		kept = append(kept, '\n')
	}
	if removed == 0 {
		return 0, nil
	}
	if err := WriteAtomic(Path(dir), kept); err != nil {
		return 0, err
	}
	return removed, nil
}

// readAll decodes every well-formed line of the log.
func readAll(dir string) ([]Entry, error) {
	lines, err := ReadLines(Path(dir))
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(lines))
	for _, line := range lines {
		var e Entry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}
