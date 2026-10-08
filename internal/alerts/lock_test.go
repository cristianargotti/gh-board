package alerts_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/audit"
)

func TestLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	release, err := alerts.Lock(dir, nil)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if _, err := alerts.Lock(dir, nil); !errors.Is(err, audit.ErrLocked) {
		t.Fatalf("the running process holds the lock, got %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(alerts.LockPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("release must remove the lock file")
	}
	again, err := alerts.Lock(dir, nil)
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	_ = again()
}

var staleCases = []struct {
	name    string
	content string
	age     time.Duration
	alive   bool
	taken   bool
}{
	{name: "a dead holder is taken over", content: "4242", alive: false, taken: true},
	{name: "a live holder keeps the lock", content: "4242\n", alive: true, taken: false},
	{name: "an old unreadable lock is taken over", content: "garbage", age: 2 * time.Hour, taken: true},
	{name: "a fresh unreadable lock keeps the lock", content: "garbage", alive: true, taken: false},
	{name: "a pid that is not positive counts as unreadable", content: "0", age: 2 * time.Hour, taken: true},
}

// plantLock writes a lock file with the content and the age of the case.
func plantLock(t *testing.T, dir, content string, age time.Duration) {
	t.Helper()
	path := alerts.LockPath(dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		old := time.Now().Add(-age)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLockStale(t *testing.T) {
	for _, tc := range staleCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			plantLock(t, dir, tc.content, tc.age)
			alive := func(int) bool { return tc.alive }
			release, err := alerts.Lock(dir, alive)
			if !tc.taken {
				if !errors.Is(err, audit.ErrLocked) {
					t.Fatalf("error = %v, want ErrLocked", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lock: %v", err)
			}
			data, _ := os.ReadFile(alerts.LockPath(dir))
			if string(data) != strconv.Itoa(os.Getpid()) {
				t.Fatalf("the lock must now hold this pid, got %q", data)
			}
			_ = release()
		})
	}
}

// TestLockAsksTheSystem exercises the default liveness probe: this process
// is alive, and a pid beyond what any system hands out is not.
func TestLockAsksTheSystem(t *testing.T) {
	dir := t.TempDir()
	plantLock(t, dir, strconv.Itoa(os.Getpid()), 0)
	if _, err := alerts.Lock(dir, nil); !errors.Is(err, audit.ErrLocked) {
		t.Fatalf("own pid must count as alive, got %v", err)
	}
	plantLock(t, dir, "2147483000", 0)
	release, err := alerts.Lock(dir, nil)
	if err != nil {
		t.Fatalf("a pid nobody has must be stale: %v", err)
	}
	_ = release()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := alerts.Lock(filepath.Join(blocker, "state"), nil); err == nil || errors.Is(err, audit.ErrLocked) {
		t.Fatalf("a directory under a file must fail, got %v", err)
	}
}
