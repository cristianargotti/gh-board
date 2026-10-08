package audit_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
)

func TestReadDir(t *testing.T) {
	dir := t.TempDir()
	if entries, err := audit.ReadDir(filepath.Join(dir, "missing", "deeper")); err != nil || entries != nil {
		t.Fatalf("missing directory = %v, %v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := audit.ReadDir(dir)
	if err != nil || len(entries) != 2 || entries[0].Name() != "a" || entries[1].Name() != "b.json" {
		t.Fatalf("ReadDir = %v, %v", entries, err)
	}
}

// blockedPath returns a path that runs through a regular file.
func blockedPath(t *testing.T) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(blocker, "deeper", "x.jsonl")
}

// pathCalls are the helpers that classify their path, by name, for the
// tables of failures every operating system must report the same way.
var pathCalls = map[string]func(path string) error{
	"ReadDir":    func(path string) error { _, err := audit.ReadDir(path); return err },
	"ReadLines":  func(path string) error { _, err := audit.ReadLines(path); return err },
	"AppendLine": func(path string) error { return audit.AppendLine(path, []byte("x")) },
}

func TestPathsThroughFile(t *testing.T) {
	path := blockedPath(t)
	for name, call := range pathCalls {
		if err := call(path); !errors.Is(err, audit.ErrNotDir) {
			t.Errorf("%s through a file = %v, want ErrNotDir", name, err)
		}
	}
	if _, err := audit.ReadDir(filepath.Dir(filepath.Dir(path))); !errors.Is(err, audit.ErrNotDir) {
		t.Errorf("ReadDir of a file = %v, want ErrNotDir", err)
	}
}

func TestPathsOnDirectory(t *testing.T) {
	dir := t.TempDir()
	for name, call := range pathCalls {
		if name == "ReadDir" {
			continue
		}
		if err := call(dir); !errors.Is(err, audit.ErrIsDir) {
			t.Errorf("%s on a directory = %v, want ErrIsDir", name, err)
		}
	}
}

func TestReadLinesUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions do not block reads here")
	}
	path := filepath.Join(t.TempDir(), "x.jsonl")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := audit.ReadLines(path)
	if err == nil || errors.Is(err, audit.ErrIsDir) || errors.Is(err, audit.ErrNotDir) {
		t.Fatalf("unreadable file = %v, want an open error", err)
	}
}
