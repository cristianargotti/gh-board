package alerts_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/audit"
)

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	old := audit.Entry{At: fxNow.Add(-audit.Retention - time.Hour), Actor: "old", Command: "move", Result: "ok"}
	recent := audit.Entry{At: fxNow.Add(-time.Hour), Actor: "recent", Command: "move", Result: "ok"}
	for _, e := range []audit.Entry{old, recent} {
		if err := audit.Append(dir, e); err != nil {
			t.Fatal(err)
		}
	}
	n, err := alerts.Prune(dir, fxNow)
	if err != nil || n != 1 {
		t.Fatalf("Prune = %d, %v; want 1", n, err)
	}
	entries, err := audit.Query(dir, time.Time{})
	if err != nil || len(entries) != 1 || entries[0].Actor != "recent" {
		t.Fatalf("entries after prune = %+v, %v", entries, err)
	}
	if n, err := alerts.Prune(t.TempDir(), fxNow); err != nil || n != 0 {
		t.Fatalf("Prune without a log = %d, %v", n, err)
	}
}
