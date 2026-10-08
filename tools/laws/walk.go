package main

import (
	"bytes"
	"io/fs"
	"path/filepath"
)

// skippedDirs are never walked: they hold history, dependencies or builds.
var skippedDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "dist": true,
}

// walkFiles visits every regular file under root that git would track,
// giving the absolute path and the slash-separated path relative to root.
// Files and directories matched by a .gitignore on the way down are
// skipped, so generated trees such as the Claude plugin types never count.
func walkFiles(root string, visit func(path, rel string) error) error {
	var stack ignoreStack
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return enterDir(root, path, rel, d, &stack)
		}
		if !d.Type().IsRegular() || stack.ignored(root, rel, false) {
			return nil
		}
		return visit(path, rel)
	})
}

// enterDir skips the fixed directories and the ignored ones, and pushes
// the .gitignore of the directory it enters.
func enterDir(root, path, rel string, d fs.DirEntry, stack *ignoreStack) error {
	if path != root && (skippedDirs[d.Name()] || stack.ignored(root, rel, true)) {
		return filepath.SkipDir
	}
	rules, err := readIgnore(path)
	if err != nil {
		return err
	}
	*stack = trimStack(*stack, root, rel)
	if len(rules.patterns) > 0 {
		*stack = append(*stack, rules)
	}
	return nil
}

// trimStack drops the rules of directories that are not ancestors of the
// directory being entered, since WalkDir gives no leave event.
func trimStack(stack ignoreStack, root, rel string) ignoreStack {
	kept := stack[:0]
	for _, r := range stack {
		base, err := filepath.Rel(root, r.dir)
		if err != nil {
			continue
		}
		base = filepath.ToSlash(base)
		if base == "." || rel == base || len(rel) > len(base) && rel[:len(base)+1] == base+"/" {
			kept = append(kept, r)
		}
	}
	return kept
}

// isBinary treats a file with a NUL byte in its first 8000 bytes as binary.
func isBinary(data []byte) bool {
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	return bytes.IndexByte(head, 0) >= 0
}
