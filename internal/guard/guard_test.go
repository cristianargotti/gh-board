package guard_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

func TestParseAgent(t *testing.T) {
	for _, in := range []string{"claude", " Cursor ", "CODEX"} {
		if _, err := guard.ParseAgent(in); err != nil {
			t.Errorf("%q: %v", in, err)
		}
	}
	if _, err := guard.ParseAgent("copilot"); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
	if len(guard.Agents) != 3 {
		t.Fatalf("Agents = %v", guard.Agents)
	}
}
