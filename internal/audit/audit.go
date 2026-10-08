// Package audit keeps the local JSONL audit log and the file helpers every
// state writer shares: atomic writes with owner-only permissions, append of
// one JSONL line and a single-instance lock (section 6.9). The log lives in
// audit.jsonl under the state directory (log.go); the helpers are in
// audit.go, lines.go and paths.go.
package audit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Audit log layout and retention.
const (
	// FileName is the audit log under the state directory.
	FileName = "audit.jsonl"
	// Retention is how long audit and journal lines are kept.
	Retention = 90 * 24 * time.Hour
	// DirPerm is the permission of every directory the kit creates.
	DirPerm = 0o700
	// FilePerm is the permission of every file the kit creates.
	FilePerm = 0o600
)

// ErrLocked is returned by Lock when another instance holds the lock.
var ErrLocked = errors.New("another instance holds the lock")

// Entry is one audit line: who did what, on which targets, with which
// outcome. Reason comes from --reason; Targets are node ids; Items are the
// same targets as owner/repo#n for a reader; Changes carry the before and
// after value of every write the command made.
type Entry struct {
	At      time.Time         `json:"at"`
	Actor   string            `json:"actor"`
	Host    string            `json:"host"`
	Project domain.ProjectRef `json:"project"`
	Command string            `json:"command"`
	Reason  string            `json:"reason,omitempty"`
	Targets []string          `json:"targets"`
	Items   []string          `json:"items,omitempty"`
	Changes []Change          `json:"changes,omitempty"`
	Result  string            `json:"result"`
	Error   string            `json:"error,omitempty"`
	DryRun  bool              `json:"dry_run"`
	PlanID  string            `json:"plan_id,omitempty"`
}

// Change is one value a write changed on a target.
type Change struct {
	Target string `json:"target"`
	Field  string `json:"field,omitempty"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// WriteAtomic writes data to path through a temporary file in the same
// directory and a rename, so readers never see a partial file. The
// directory is created with DirPerm and the file with FilePerm.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("temporary file in %s: %w", dir, err)
	}
	name := tmp.Name()
	err = writeAndClose(tmp, data)
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// writeAndClose sets the permissions, writes, syncs and closes the file,
// returning the first failure.
func writeAndClose(f *os.File, data []byte) error {
	err := f.Chmod(FilePerm)
	if err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Lock creates path exclusively and returns the release function. A second
// caller gets ErrLocked while the file exists; the file holds the pid of
// the holder for diagnostics. Stale locks are for the caller to handle.
func Lock(path string) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(path), DirPerm); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FilePerm) //nolint:gosec // lock file under the state directory
	if errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("%s: %w", path, ErrLocked)
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = unlock(f, path)
		return nil, err
	}
	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() { releaseErr = unlock(f, path) })
		return releaseErr
	}
	return release, nil
}

// unlock closes the handle and removes the lock file.
func unlock(f *os.File, path string) error {
	closeErr := f.Close()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return closeErr
}
