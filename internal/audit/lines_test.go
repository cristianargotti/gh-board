package audit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
)

var appendCases = []struct {
	name  string
	lines []string
	want  string
}{
	{name: "adds the newline", lines: []string{`{"a":1}`}, want: "{\"a\":1}\n"},
	{name: "keeps a given newline", lines: []string{"{\"a\":1}\n"}, want: "{\"a\":1}\n"},
	{name: "appends in order", lines: []string{"one", "two"}, want: "one\ntwo\n"},
}

// assertFile checks the content and the owner-only permission of a file.
func assertFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("content = %q, %v; want %q", data, err, want)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != audit.FilePerm {
		t.Fatalf("permissions = %v, %v", info.Mode().Perm(), err)
	}
}

func TestAppendLine(t *testing.T) {
	for _, tc := range appendCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "audit.jsonl")
			for _, line := range tc.lines {
				if err := audit.AppendLine(path, []byte(line)); err != nil {
					t.Fatalf("AppendLine: %v", err)
				}
			}
			assertFile(t, path, tc.want)
		})
	}
}

func TestAppendLineFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := audit.AppendLine(filepath.Join(blocker, "x.jsonl"), []byte("x")); err == nil {
		t.Fatal("a directory under a file must fail")
	}
	if err := audit.AppendLine(t.TempDir(), []byte("x")); err == nil {
		t.Fatal("appending to a directory must fail")
	}
}

var readLinesCases = []struct {
	name    string
	content string
	want    []string
}{
	{name: "missing file", content: "", want: nil},
	{name: "skips blank lines", content: "a\n\n  \nb\n", want: []string{"a", "b"}},
	{name: "trims", content: " a \r\nb", want: []string{"a", "b"}},
}

// assertLines compares the lines read with the expected strings.
func assertLines(t *testing.T, got [][]byte, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d", len(got), len(want))
	}
	for i, line := range got {
		if string(line) != want[i] {
			t.Errorf("line %d = %q, want %q", i, line, want[i])
		}
	}
}

func TestReadLines(t *testing.T) {
	for _, tc := range readLinesCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x.jsonl")
			if tc.content != "" {
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			lines, err := audit.ReadLines(path)
			if err != nil {
				t.Fatalf("ReadLines: %v", err)
			}
			assertLines(t, lines, tc.want)
		})
	}
}

func TestReadLinesFailures(t *testing.T) {
	if _, err := audit.ReadLines(t.TempDir()); err == nil {
		t.Fatal("reading a directory must fail")
	}
	path := filepath.Join(t.TempDir(), "long.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", audit.MaxLineBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := audit.ReadLines(path); err == nil {
		t.Fatal("a line over the limit must fail")
	}
}
