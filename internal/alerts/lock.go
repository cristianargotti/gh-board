package alerts

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
)

// StaleLockAge is how old a lock whose holder cannot be identified must be
// before watch takes it over.
const StaleLockAge = time.Hour

// Lock takes the single-instance lock of watch under the state directory
// through audit.Lock. A lock left behind by a process that no longer runs
// is stale and taken over; alive reports whether a pid runs, nil asks the
// operating system. The release removes the file.
func Lock(stateDir string, alive func(pid int) bool) (func() error, error) {
	if alive == nil {
		alive = processAlive
	}
	path := LockPath(stateDir)
	release, err := audit.Lock(path)
	if !errors.Is(err, audit.ErrLocked) {
		return release, err
	}
	if !stale(path, alive) {
		return nil, fmt.Errorf("watch is already running: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stale lock %s: %w", path, err)
	}
	return audit.Lock(path)
}

// stale reads the holder's pid: a dead holder means the lock was left
// behind, and so does an unreadable holder older than StaleLockAge.
func stale(path string, alive func(int) bool) bool {
	data, err := os.ReadFile(path) //nolint:gosec // lock file under the state directory
	if err != nil {
		return false
	}
	if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil && pid > 0 {
		return !alive(pid)
	}
	info, err := os.Stat(path)
	return err == nil && time.Since(info.ModTime()) > StaleLockAge
}

// processAlive asks the operating system whether pid runs. On Unix the
// null signal probes without effect and a permission error still means a
// live process of another user; on Windows finding the process is the
// probe, because signals are not supported there.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || !errors.Is(err, os.ErrProcessDone)
}
