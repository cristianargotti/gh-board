package commands

import (
	"embed"
	"errors"
	"testing"

	assets "github.com/cristianargotti/gh-board"
)

// An empty embed.FS stands in for a build whose agents directory did not
// ship, the one condition under which the skill asset is missing.
func TestAutoPlanCursorWithoutSkillAsset(t *testing.T) {
	saved := assets.FS
	assets.FS = embed.FS{}
	t.Cleanup(func() { assets.FS = saved })
	if _, err := autoPlanCursor(autoScopeUser, false, autoRoots{Home: t.TempDir()}); err == nil {
		t.Fatal("expected the missing skill asset to fail the plan")
	}
}

func TestAutoHookFragmentReportsGeneratorErrors(t *testing.T) {
	boom := errors.New("boom")
	gen := func(bool) ([]byte, error) { return nil, boom }
	if _, err := autoHookFragment(gen, false, "cursor hook"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}
