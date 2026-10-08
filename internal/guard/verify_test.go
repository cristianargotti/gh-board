package guard_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

func TestSHA256(t *testing.T) {
	if got := guard.SHA256(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("SHA256(nil) = %s", got)
	}
}

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "hooks.json")
	if err := os.WriteFile(ok, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := guard.SHA256([]byte("content"))
	results, err := guard.Verify([]guard.Artifact{
		{Path: ok, SHA256: sum},
		{Path: ok, SHA256: strings.ToUpper(sum)},
		{Path: ok, SHA256: guard.SHA256([]byte("other"))},
		{Path: filepath.Join(dir, "missing"), SHA256: sum},
		{Path: dir, SHA256: sum},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantReasons := []string{"", "", "modified: sha256 ", "missing", "unreadable: "}
	for i, r := range results {
		if r.OK != (wantReasons[i] == "") || !strings.HasPrefix(r.Reason, wantReasons[i]) {
			t.Errorf("result %d: %+v, want reason prefix %q", i, r, wantReasons[i])
		}
	}
	if _, err := guard.Verify([]guard.Artifact{{SHA256: sum}}); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("empty path: %v", err)
	}
}
