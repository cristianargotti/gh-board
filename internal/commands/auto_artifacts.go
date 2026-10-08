package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoKind separates the guard layer from the skill layer, so that guard
// install|uninstall touch only the former (section 7).
type autoKind string

// Artifact kinds.
const (
	autoKindGuard autoKind = "guard"
	autoKindSkill autoKind = "skill"
	autoKindMCP   autoKind = "mcp"
)

// autoAllKinds is what agent install|uninstall handle; the MCP entry is
// planned only when --mcp asks for it and removed with the rest.
var autoAllKinds = []autoKind{autoKindGuard, autoKindSkill, autoKindMCP}

// autoArtifactType is how the installer changes a file.
type autoArtifactType string

// Artifact types.
const (
	// autoTypeFile is a whole file the kit owns.
	autoTypeFile autoArtifactType = "file"
	// autoTypeJSON is a fragment merged into a JSON object the agent owns.
	autoTypeJSON autoArtifactType = "json"
	// autoTypeBlock is a marked block kept inside a text file the team owns.
	autoTypeBlock autoArtifactType = "block"
	// autoTypeTOML is a marked block of TOML tables appended to a file the
	// agent owns (the Codex configuration).
	autoTypeTOML autoArtifactType = "toml"
)

// Actions an install or uninstall reports per file.
const (
	autoActionCreated   = "created"
	autoActionUpdated   = "updated"
	autoActionUnchanged = "unchanged"
	autoActionRemoved   = "removed"
	autoActionKept      = "kept"
	autoActionAbsent    = "absent"
)

// autoArtifact is one planned change of an installer.
type autoArtifact struct {
	Kind autoKind
	Type autoArtifactType
	Path string
	// Content is the whole file (file) or the block body (block).
	Content []byte
	// Fragment is the object merged into the file (json).
	Fragment map[string]any
	// OwnDir marks a file whose parent directory exists only for it, so
	// that uninstall removes the directory too.
	OwnDir bool
}

// autoResult is what applying one artifact or receipt entry produced.
type autoResult struct {
	Path   string
	Action string
	Entry  autoReceiptEntry
	Notes  []string
}

// autoWorkspace reads and writes the files of one run. Writes go through
// an overlay so that two artifacts on the same file see each other and so
// that --dry-run shows every diff without touching the disk.
type autoWorkspace struct {
	dryRun  bool
	out     io.Writer
	overlay map[string][]byte
	exists  map[string]bool
}

func autoNewWorkspace(out io.Writer, dryRun bool) *autoWorkspace {
	return &autoWorkspace{dryRun: dryRun, out: out, overlay: map[string][]byte{}, exists: map[string]bool{}}
}

// read returns the current content of a file and whether it exists.
func (w *autoWorkspace) read(path string) ([]byte, bool, error) {
	if data, ok := w.overlay[path]; ok {
		return data, w.exists[path], nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // agent files under the home or the working directory
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, nil
}

// write prints the diff and stores the new content, on disk unless the run
// is a dry run.
func (w *autoWorkspace) write(path string, before, after []byte, existed bool) error {
	if diff := autoDiff(path, before, after); diff != "" {
		_, _ = io.WriteString(w.out, diff)
	}
	w.overlay[path], w.exists[path] = after, true
	if w.dryRun {
		return nil
	}
	return autoWriteFile(path, after, existed)
}

// remove deletes a file after printing its removal as a diff.
func (w *autoWorkspace) remove(path string, before []byte, ownDir bool) error {
	if diff := autoDiff(path, before, nil); diff != "" {
		_, _ = io.WriteString(w.out, diff)
	}
	w.overlay[path], w.exists[path] = nil, false
	if w.dryRun {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	if ownDir {
		// Best effort: the directory stays when anything else lives in it.
		_ = os.Remove(filepath.Dir(path))
	}
	return nil
}

// autoWriteFile writes atomically with owner-only permissions for a new
// file and the existing mode for a file the agent or the team created.
func autoWriteFile(path string, data []byte, existed bool) error {
	mode := os.FileMode(audit.FilePerm)
	if existed {
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	}
	if err := audit.WriteAtomic(path, data); err != nil {
		return err
	}
	if mode == audit.FilePerm {
		return nil
	}
	return os.Chmod(path, mode)
}

// install applies one artifact and returns the receipt entry to record.
func (w *autoWorkspace) install(a autoArtifact) (autoResult, error) {
	before, existed, err := w.read(a.Path)
	if err != nil {
		return autoResult{}, err
	}
	after, delta, notes, err := autoApplyArtifact(a, before)
	if err != nil {
		return autoResult{}, err
	}
	res := autoResult{Path: a.Path, Notes: notes, Entry: autoReceiptEntry{
		Kind: a.Kind, Type: a.Type, Path: a.Path, Created: !existed, OwnDir: a.OwnDir, Delta: delta,
	}}
	if existed && bytes.Equal(before, after) {
		res.Action, res.Entry.SHA256 = autoActionUnchanged, guard.SHA256(after)
		return res, nil
	}
	if err := w.write(a.Path, before, after, existed); err != nil {
		return autoResult{}, err
	}
	res.Action, res.Entry.SHA256 = autoActionUpdated, guard.SHA256(after)
	if !existed {
		res.Action = autoActionCreated
	}
	return res, nil
}

// autoApplyArtifact computes the new content of a file for an artifact and
// the delta a JSON merge added, which the receipt keeps for uninstall.
func autoApplyArtifact(a autoArtifact, before []byte) ([]byte, map[string]any, []string, error) {
	switch a.Type {
	case autoTypeFile:
		return a.Content, nil, nil, nil
	case autoTypeBlock:
		return autoUpsertBlock(before, a.Content), nil, nil, nil
	case autoTypeTOML:
		return autoApplyTOML(a, before)
	case autoTypeJSON:
		obj, err := autoJSONObject(before, a.Path)
		if err != nil {
			return nil, nil, nil, err
		}
		var notes []string
		for _, c := range autoJSONConflicts(obj, a.Fragment, "") {
			notes = append(notes, fmt.Sprintf("%s already set to another value, left as is", c))
		}
		delta := autoJSONDelta(obj, a.Fragment)
		if delta == nil {
			return before, nil, notes, nil
		}
		after, err := autoJSONEncode(autoJSONMerge(obj, delta))
		return after, delta, notes, err
	}
	return nil, nil, nil, fmt.Errorf("artifact type %q is unknown", a.Type)
}

// uninstall reverts one receipt entry.
func (w *autoWorkspace) uninstall(e autoReceiptEntry) (autoResult, error) {
	before, existed, err := w.read(e.Path)
	if err != nil {
		return autoResult{}, err
	}
	res := autoResult{Path: e.Path, Entry: e, Action: autoActionAbsent}
	if !existed {
		return res, nil
	}
	after, empty, err := autoRevertArtifact(e, before)
	if err != nil {
		return autoResult{}, err
	}
	switch {
	case e.Type == autoTypeFile || (empty && e.Created):
		res.Action = autoActionRemoved
		return res, w.remove(e.Path, before, e.OwnDir)
	case bytes.Equal(before, after):
		res.Action = autoActionKept
		return res, nil
	}
	res.Action = autoActionUpdated
	return res, w.write(e.Path, before, after, true)
}

// autoRevertArtifact computes the content of a file without what the
// receipt entry added and whether nothing of substance remains.
func autoRevertArtifact(e autoReceiptEntry, before []byte) ([]byte, bool, error) {
	switch e.Type {
	case autoTypeFile:
		return nil, true, nil
	case autoTypeBlock:
		after := autoRemoveBlock(before)
		return after, len(bytes.TrimSpace(after)) == 0, nil
	case autoTypeTOML:
		after := autoRemoveBlockWith(before, autoTOMLMarkers)
		return after, len(bytes.TrimSpace(after)) == 0, nil
	case autoTypeJSON:
		obj, err := autoJSONObject(before, e.Path)
		if err != nil {
			return nil, false, err
		}
		original := autoJSONClone(obj)
		autoJSONUnmerge(obj, e.Delta)
		if reflect.DeepEqual(original, obj) {
			return before, len(obj) == 0, nil
		}
		after, err := autoJSONEncode(obj)
		return after, len(obj) == 0, err
	}
	return nil, false, fmt.Errorf("artifact type %q is unknown", e.Type)
}
