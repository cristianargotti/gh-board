package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// forbiddenPhrases are matched case-insensitively in commit messages.
var forbiddenPhrases = []string{"co-authored-by", "generated with"}

// checkCommits reads the last commits of the repository at root. A root
// without .git, or a repository without commits, has nothing to check.
func checkCommits(root string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, nil
	}
	cmd := exec.Command("git", "log", "-n", commitWindow, "--format=%H%x1f%B%x1e")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if unbornBranch(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("git log: %w", err)
	}
	return commitViolations(string(out)), nil
}

// unbornBranch recognizes the git error of a branch with no commits.
func unbornBranch(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && bytes.Contains(exitErr.Stderr, []byte("does not have any commits"))
}

// commitViolations parses records of "<hash>\x1f<message>\x1e".
func commitViolations(log string) []string {
	var out []string
	for _, record := range strings.Split(log, "\x1e") {
		hash, message, ok := strings.Cut(record, "\x1f")
		if !ok {
			continue
		}
		lower := strings.ToLower(message)
		for _, phrase := range forbiddenPhrases {
			if strings.Contains(lower, phrase) {
				out = append(out, fmt.Sprintf("commit %s: message contains %q", shortHash(hash), phrase))
			}
		}
	}
	return out
}

func shortHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
