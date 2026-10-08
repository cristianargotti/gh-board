package github

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAtomicFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(filepath.Join(blocker, "schema.json"), []byte("{}")); err == nil {
		t.Fatal("a parent that is a file must fail")
	}
	if runtime.GOOS != "windows" && os.Getuid() != 0 {
		readOnly := filepath.Join(t.TempDir(), "ro")
		if err := os.Mkdir(readOnly, 0o500); err != nil {
			t.Fatal(err)
		}
		if err := writeAtomic(filepath.Join(readOnly, "schema.json"), []byte("{}")); err == nil {
			t.Fatal("a read-only directory must fail")
		}
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeAndClose(closed, []byte("{}")); err == nil {
		t.Fatal("writing a closed file must fail")
	}
	good := filepath.Join(t.TempDir(), "nested", "schema.json")
	if err := writeAtomic(good, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(good); err != nil || string(data) != "{}" {
		t.Fatalf("read back %q, %v", data, err)
	}
}
