package audit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

var (
	t0      = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	project = domain.ProjectRef{Owner: "acme", Number: 7}
)

func entry(at time.Time, command string) audit.Entry {
	return audit.Entry{
		At: at, Actor: "octocat", Host: "github.com", Project: project,
		Command: command, Targets: []string{"I_kwDOA"}, Result: "ok",
	}
}

func seed(t *testing.T, entries ...audit.Entry) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	for _, e := range entries {
		if err := audit.Append(dir, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	return dir
}

func TestAppendStampsTime(t *testing.T) {
	dir := seed(t, audit.Entry{Command: "move", Result: "ok"})
	entries, err := audit.Query(dir, time.Time{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("Query = %v, %v", entries, err)
	}
	if entries[0].At.IsZero() || entries[0].Command != "move" {
		t.Fatalf("entry = %+v", entries[0])
	}
}

var queryCases = []struct {
	name  string
	since time.Time
	want  []string
}{
	{name: "everything", since: time.Time{}, want: []string{"first", "second", "third"}},
	{name: "since the second", since: t0.Add(24 * time.Hour), want: []string{"second", "third"}},
	{name: "nothing", since: t0.Add(72 * time.Hour), want: nil},
}

func TestQuery(t *testing.T) {
	dir := seed(t,
		entry(t0.Add(48*time.Hour), "third"),
		entry(t0, "first"),
		entry(t0.Add(24*time.Hour), "second"),
	)
	for _, tc := range queryCases {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := audit.Query(dir, tc.since)
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(entries) != len(tc.want) {
				t.Fatalf("got %d entries, want %d", len(entries), len(tc.want))
			}
			for i, e := range entries {
				if e.Command != tc.want[i] {
					t.Errorf("entry %d = %q, want %q", i, e.Command, tc.want[i])
				}
			}
		})
	}
}

func TestQueryMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	if entries, err := audit.Query(dir, time.Time{}); err != nil || len(entries) != 0 {
		t.Fatalf("missing log: %v, %v", entries, err)
	}
	if err := audit.AppendLine(audit.Path(dir), []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	if err := audit.Append(dir, entry(t0, "move")); err != nil {
		t.Fatal(err)
	}
	entries, err := audit.Query(dir, time.Time{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("corrupt line must be skipped: %v, %v", entries, err)
	}
	if _, err := audit.Query(filepath.Dir(audit.Path(dir)+"/x"), time.Time{}); err == nil {
		t.Fatal("a log path that is a directory must fail")
	}
}

var pruneCases = []struct {
	name    string
	before  time.Time
	removed int
	left    int
}{
	{name: "nothing old", before: t0, removed: 0, left: 3},
	{name: "one old", before: t0.Add(time.Hour), removed: 1, left: 2},
	{name: "all old", before: t0.Add(72 * time.Hour), removed: 3, left: 0},
}

func TestPrune(t *testing.T) {
	for _, tc := range pruneCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := seed(t, entry(t0, "a"), entry(t0.Add(24*time.Hour), "b"), entry(t0.Add(48*time.Hour), "c"))
			removed, err := audit.Prune(dir, tc.before)
			if err != nil || removed != tc.removed {
				t.Fatalf("Prune = %d, %v; want %d", removed, err, tc.removed)
			}
			entries, err := audit.Query(dir, time.Time{})
			if err != nil || len(entries) != tc.left {
				t.Fatalf("left %d entries, %v; want %d", len(entries), err, tc.left)
			}
		})
	}
}

func TestPruneCorruptAndMissing(t *testing.T) {
	dir := t.TempDir()
	if removed, err := audit.Prune(dir, t0); err != nil || removed != 0 {
		t.Fatalf("missing log: %d, %v", removed, err)
	}
	if err := audit.AppendLine(audit.Path(dir), []byte("{broken")); err != nil {
		t.Fatal(err)
	}
	if err := audit.Append(dir, entry(t0.Add(time.Hour), "keep")); err != nil {
		t.Fatal(err)
	}
	removed, err := audit.Prune(dir, t0)
	if err != nil || removed != 1 {
		t.Fatalf("Prune = %d, %v; want the corrupt line removed", removed, err)
	}
	data, _ := os.ReadFile(audit.Path(dir))
	if strings.Count(string(data), "\n") != 1 || !strings.Contains(string(data), "keep") {
		t.Fatalf("rewritten log = %q", data)
	}
	if _, err := audit.Prune(filepath.Join(dir, "audit.jsonl"), t0); err == nil {
		t.Fatal("a log under a file must fail")
	}
}
