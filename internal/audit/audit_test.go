package audit_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
)

func TestWriteAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "nested")
	path := filepath.Join(dir, "alerts.json")
	if err := audit.WriteAtomic(path, []byte("one")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := audit.WriteAtomic(path, []byte("two")); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "two" {
		t.Fatalf("content = %q, %v", data, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %d entries", len(entries))
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != audit.FilePerm {
			t.Fatalf("permissions = %o", info.Mode().Perm())
		}
	}
}

// readOnlyDir returns a directory the test cannot write into, or skips
// where permissions do not block writes (Windows, root).
func readOnlyDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("read-only directories do not block writes here")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestWriteAtomicFailures(t *testing.T) {
	dir := t.TempDir()
	if err := audit.WriteAtomic(dir, []byte("x")); err == nil {
		t.Fatal("renaming over a directory must fail")
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Fatalf("temporary file left behind after a failed rename: %d entries", len(entries))
	}
	ro := readOnlyDir(t)
	if err := audit.WriteAtomic(filepath.Join(ro, "x.json"), []byte("x")); err == nil {
		t.Fatal("a read-only directory must fail")
	}
}

func TestLockReadOnlyDir(t *testing.T) {
	ro := readOnlyDir(t)
	if _, err := audit.Lock(filepath.Join(ro, "x.lock")); err == nil || errors.Is(err, audit.ErrLocked) {
		t.Fatalf("expected a permission error, got %v", err)
	}
}

func TestWriteAtomicFailsUnderFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := audit.WriteAtomic(filepath.Join(blocker, "child.json"), []byte("x")); err == nil {
		t.Fatal("a directory under a file must fail")
	}
}

func TestLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locks", "watch.lock")
	release, err := audit.Lock(path)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if _, err := audit.Lock(path); !errors.Is(err, audit.ErrLocked) {
		t.Fatalf("second Lock must report ErrLocked, got %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("release must remove the lock file")
	}
	again, err := audit.Lock(path)
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	if err := again(); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if err := again(); err != nil {
		t.Fatalf("releasing twice must be harmless: %v", err)
	}
}

func TestLockFailsUnderFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := audit.Lock(filepath.Join(blocker, "x.lock")); err == nil || errors.Is(err, audit.ErrLocked) {
		t.Fatalf("expected a filesystem error, got %v", err)
	}
}
