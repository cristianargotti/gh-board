package audit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Errors the path checks wrap. The helpers decide what a path is with
// os.Stat and IsDir instead of the error of open or read, because the
// operating systems disagree: Linux and macOS answer ENOTDIR for a path
// that runs through a regular file, while Windows answers
// ERROR_PATH_NOT_FOUND, which Go maps to syscall.ENOTDIR and to
// fs.ErrNotExist alike, so a file in the way would pass for a file not
// yet written.
var (
	// ErrIsDir marks a file path that names a directory.
	ErrIsDir = errors.New("is a directory")
	// ErrNotDir marks a directory path that names a file or runs through one.
	ErrNotDir = errors.New("not a directory")
)

// ReadDir returns the entries of a directory in name order. A missing
// directory yields no entries and no error, so a state directory that
// was never written reads as empty; a path that names a file, or runs
// through one, fails on every operating system alike.
func ReadDir(path string) ([]os.DirEntry, error) {
	exists, dir, err := classify(path)
	if err != nil || !exists {
		return nil, err
	}
	if !dir {
		return nil, pathError(path, ErrNotDir)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return entries, nil
}

// statFile reports whether a path that must be a regular file exists. A
// directory at the path, or a file on the way to it, is an error.
func statFile(path string) (bool, error) {
	exists, dir, err := classify(path)
	if err != nil {
		return false, err
	}
	if dir {
		return false, pathError(path, ErrIsDir)
	}
	return exists, nil
}

// classify tells whether a path exists and whether it is a directory. A
// missing path only counts as missing when its nearest existing ancestor
// is a directory; otherwise the file in the way is the error.
func classify(path string) (exists, dir bool, err error) {
	info, err := os.Stat(path)
	if err == nil {
		return true, info.IsDir(), nil
	}
	if !missing(err) {
		return false, false, fmt.Errorf("stat %s: %w", path, err)
	}
	return false, false, checkAncestors(path)
}

// checkAncestors walks up from the parent of path to the nearest ancestor
// that exists and fails when it is not a directory.
func checkAncestors(path string) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		info, err := os.Stat(dir)
		switch {
		case err == nil && info.IsDir():
			return nil
		case err == nil:
			return pathError(dir, ErrNotDir)
		case !missing(err):
			return fmt.Errorf("stat %s: %w", dir, err)
		case filepath.Dir(dir) == dir:
			return nil
		}
	}
}

// missing reports the errors of a path that is not there, or not there
// as a directory on the way: fs.ErrNotExist everywhere and ENOTDIR,
// which Linux and macOS return through a regular file.
func missing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

func pathError(path string, kind error) error {
	return fmt.Errorf("%s: %w", path, kind)
}
