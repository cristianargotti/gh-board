package main

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignoreRules are the patterns of one .gitignore file, anchored at the
// directory that holds it. The subset understood is what this repository
// and the tools that generate files into it use: blank lines and comments
// are skipped, a trailing slash matches directories only, a leading slash
// anchors the pattern at the file's directory, a pattern with another
// slash matches the relative path, any other pattern matches a base name
// at any depth, and the glob syntax is that of path.Match. Negations are
// not supported and are skipped.
type ignoreRules struct {
	dir      string
	patterns []ignorePattern
}

type ignorePattern struct {
	text     string
	anchored bool
	dirOnly  bool
}

// readIgnore parses the .gitignore of a directory; a missing file means
// no rules.
func readIgnore(dir string) (ignoreRules, error) {
	rules := ignoreRules{dir: dir}
	f, err := os.Open(filepath.Join(dir, ".gitignore")) //nolint:gosec // the tool reads the repository it checks
	if err != nil {
		if os.IsNotExist(err) {
			return rules, nil
		}
		return rules, err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if p, ok := parseIgnoreLine(scanner.Text()); ok {
			rules.patterns = append(rules.patterns, p)
		}
	}
	return rules, scanner.Err()
}

// parseIgnoreLine reads one line; false for blanks, comments and
// negations.
func parseIgnoreLine(line string) (ignorePattern, bool) {
	line = strings.TrimRight(line, " \t\r")
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
		return ignorePattern{}, false
	}
	p := ignorePattern{}
	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		p.anchored = true
		line = strings.TrimPrefix(line, "/")
	} else if strings.Contains(line, "/") {
		p.anchored = true
	}
	p.text = line
	return p, line != ""
}

// matches reports whether the slash-separated path relative to the rules
// directory is ignored.
func (r ignoreRules) matches(rel string, isDir bool) bool {
	for _, p := range r.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		if p.matches(rel) {
			return true
		}
	}
	return false
}

func (p ignorePattern) matches(rel string) bool {
	if p.anchored {
		ok, _ := path.Match(p.text, rel)
		return ok
	}
	ok, _ := path.Match(p.text, path.Base(rel))
	return ok
}

// ignoreStack holds the rules of every directory on the way down from
// the root, so a nested .gitignore applies to its subtree only.
type ignoreStack []ignoreRules

// ignored reports whether path, relative to root, is ignored by any rules
// on the stack.
func (s ignoreStack) ignored(root, rel string, isDir bool) bool {
	for _, r := range s {
		base, err := filepath.Rel(root, r.dir)
		if err != nil {
			continue
		}
		base = filepath.ToSlash(base)
		sub := rel
		if base != "." {
			if !strings.HasPrefix(rel, base+"/") {
				continue
			}
			sub = strings.TrimPrefix(rel, base+"/")
		}
		if r.matches(sub, isDir) {
			return true
		}
	}
	return false
}
