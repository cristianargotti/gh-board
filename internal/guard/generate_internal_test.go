package guard

import (
	"errors"
	"path/filepath"
	"testing"
)

var binaryDependents = map[string]func() error{
	"Binary":      func() error { _, err := Binary(); return err },
	"HookCommand": func() error { _, err := HookCommand(AgentClaude, false); return err },
	"CursorHook":  func() error { _, err := CursorHook(false); return err },
	"CodexHook":   func() error { _, err := CodexHook(true); return err },
	"ClaudeHook":  func() error { _, err := ClaudeHook(false); return err },
}

func TestBinaryFailure(t *testing.T) {
	orig := executable
	t.Cleanup(func() { executable = orig })
	executable = func() (string, error) { return "", errors.New("boom") }
	for name, fn := range binaryDependents {
		if err := fn(); err == nil {
			t.Errorf("%s: expected the resolution error", name)
		}
	}
	executable = func() (string, error) { return filepath.Join(t.TempDir(), "missing"), nil }
	if _, err := Binary(); err == nil {
		t.Error("expected an error for a binary that does not exist")
	}
}
