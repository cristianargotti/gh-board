//go:build windows

package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPipeNameOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if name, ok := pipeName(f); !ok || name == "" || isCygwinTerminal(f) {
		t.Fatalf("a regular file has a name (%q, %v) and is not a pty", name, ok)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	if isCygwinTerminal(r) {
		t.Fatal("an anonymous pipe is not a pty")
	}
	if got := fileNameOf([]uint16{4, 0, 'a', 'b', 'c'}); got != "ab" {
		t.Fatalf("fileNameOf = %q", got)
	}
}

// The name query fails on a closed handle, and a name longer than the
// buffer is clamped to it.
func TestPipeNameFailuresOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if name, ok := pipeName(f); ok || name != "" {
		t.Fatalf("a closed handle has no name, got %q, %v", name, ok)
	}
	if got := fileNameOf([]uint16{40, 0, 'a'}); got != "a" {
		t.Fatalf("fileNameOf must clamp to the buffer, got %q", got)
	}
}
