package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoReceiptsDir holds one receipt per agent and scope under the config
// directory, so that uninstall removes exactly what install added and
// doctor can verify every file (section 6.5).
const autoReceiptsDir = "agents"

// autoReceiptEntry records one file an installer touched.
type autoReceiptEntry struct {
	Kind    autoKind         `json:"kind"`
	Type    autoArtifactType `json:"type"`
	Path    string           `json:"path"`
	Created bool             `json:"created"`
	OwnDir  bool             `json:"own_dir,omitempty"`
	Delta   map[string]any   `json:"delta,omitempty"`
	SHA256  string           `json:"sha256"`
}

// autoReceipt is the install record of one agent in one scope.
type autoReceipt struct {
	Agent       guard.Agent        `json:"agent"`
	Scope       autoScope          `json:"scope"`
	Strict      bool               `json:"strict"`
	Version     string             `json:"version"`
	InstalledAt time.Time          `json:"installed_at"`
	Entries     []autoReceiptEntry `json:"artifacts"`
}

// autoReceiptPath is <config dir>/agents/<agent>-<scope>.json.
func autoReceiptPath(dirs config.Dirs, agent guard.Agent, scope autoScope) string {
	return filepath.Join(dirs.Config, autoReceiptsDir, fmt.Sprintf("%s-%s.json", agent, scope))
}

// autoReadReceipt loads a receipt; found is false when none exists.
func autoReadReceipt(path string) (autoReceipt, bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // receipt under the kit config directory
	if errors.Is(err, fs.ErrNotExist) {
		return autoReceipt{}, false, nil
	}
	if err != nil {
		return autoReceipt{}, false, fmt.Errorf("read receipt %s: %w", path, err)
	}
	var r autoReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return autoReceipt{}, false, fmt.Errorf("receipt %s is damaged: %w", path, err)
	}
	return r, true, nil
}

// autoWriteReceipt stores a receipt atomically.
func autoWriteReceipt(path string, r autoReceipt) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return audit.WriteAtomic(path, append(data, '\n'))
}

// autoDeleteReceipt removes a receipt; a missing one is fine.
func autoDeleteReceipt(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove receipt %s: %w", path, err)
	}
	return nil
}

// upsert records an entry. The same path and kind is replaced, keeping
// Created from the first install (the file was ours from the start) and
// the union of the deltas, so that a later --strict install adds to what
// uninstall will remove.
func (r *autoReceipt) upsert(e autoReceiptEntry) {
	r.setHash(e.Path, e.SHA256)
	for i, old := range r.Entries {
		if old.Path != e.Path || old.Kind != e.Kind {
			continue
		}
		e.Created = old.Created
		if old.Delta != nil {
			e.Delta = autoJSONMerge(autoJSONClone(old.Delta).(map[string]any), e.Delta)
		}
		r.Entries[i] = e
		return
	}
	r.Entries = append(r.Entries, e)
}

// Several layers share settings.json, so every entry must describe its final contents.
func (r *autoReceipt) setHash(path, hash string) {
	for i := range r.Entries {
		if r.Entries[i].Path == path {
			r.Entries[i].SHA256 = hash
		}
	}
}

// Removing one layer changes the hash of layers retained in the same file.
func (r *autoReceipt) refreshWrittenHashes(ws *autoWorkspace) {
	for i := range r.Entries {
		if data, ok := ws.overlay[r.Entries[i].Path]; ok {
			r.Entries[i].SHA256 = guard.SHA256(data)
		}
	}
}

// split separates the entries of the kinds from the rest.
func (r autoReceipt) split(kinds []autoKind) (chosen, rest []autoReceiptEntry) {
	for _, e := range r.Entries {
		if autoKindIn(kinds, e.Kind) {
			chosen = append(chosen, e)
		} else {
			rest = append(rest, e)
		}
	}
	return chosen, rest
}

// createdFiles lists the files the kit created: several entries may share
// a file (the Claude settings) and only the first one saw it absent.
func (r autoReceipt) createdFiles() map[string]bool {
	out := map[string]bool{}
	for _, e := range r.Entries {
		if e.Created {
			out[e.Path] = true
		}
	}
	return out
}

func autoKindIn(kinds []autoKind, kind autoKind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// autoInstalledArtifacts lists every file the installers recorded with the
// hash they left, for doctor to verify through guard.Verify. Files the
// agent also edits (settings and hooks) change hash legitimately; their
// entries are listed last so doctor can report them apart.
func autoInstalledArtifacts(dirs config.Dirs) ([]guard.Artifact, error) {
	pattern := filepath.Join(dirs.Config, autoReceiptsDir, "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var whole, merged []guard.Artifact
	for _, p := range paths {
		r, found, err := autoReadReceipt(p)
		if err != nil || !found {
			return nil, err
		}
		for _, e := range r.Entries {
			if e.Kind == autoKindMCP {
				// The agent rewrites these files itself; doctor checks the
				// entry they hold instead of the hash of the whole file.
				continue
			}
			a := guard.Artifact{Path: e.Path, SHA256: e.SHA256}
			if e.Type == autoTypeJSON {
				merged = append(merged, a)
			} else {
				whole = append(whole, a)
			}
		}
	}
	return append(whole, merged...), nil
}
