package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// StatePath returns alerts.json under the state directory.
func StatePath(stateDir string) string { return filepath.Join(stateDir, StateFileName) }

// LockPath returns the watch lock under the state directory.
func LockPath(stateDir string) string { return filepath.Join(stateDir, LockFileName) }

// ReadState reads alerts.json. A missing file is an empty state; a file
// that does not decode comes back as an error with an empty state, so the
// caller decides whether to start over.
func ReadState(path string) (domain.AlertState, error) {
	data, err := os.ReadFile(path) //nolint:gosec // state file under the user directory
	if errors.Is(err, os.ErrNotExist) {
		return domain.AlertState{}, nil
	}
	if err != nil {
		return domain.AlertState{}, fmt.Errorf("read %s: %w", path, err)
	}
	var state domain.AlertState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.AlertState{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return state, nil
}

// WriteState writes alerts.json atomically with owner-only permissions, in
// the shape testdata/alerts.example.json documents. The alerts list is
// never null, because the mod rejects a state without an array.
func WriteState(path string, state domain.AlertState) error {
	if state.Alerts == nil {
		state.Alerts = []domain.Alert{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode alert state: %w", err)
	}
	return audit.WriteAtomic(path, append(data, '\n'))
}
