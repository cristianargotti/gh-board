package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// fileCoverage accumulates the statements of one file. Blocks are keyed by
// their range so that a block reported by several test binaries counts
// once, covered when any run executed it.
type fileCoverage struct {
	blocks map[string]block
}

type block struct {
	stmts   int
	covered bool
}

// totals returns the statements and the covered statements of the file.
func (fc *fileCoverage) totals() (stmts, covered int) {
	for _, b := range fc.blocks {
		stmts += b.stmts
		if b.covered {
			covered += b.stmts
		}
	}
	return stmts, covered
}

// parseProfile reads the "mode:" header and the block lines
// "<file>:<start>,<end> <statements> <count>".
func parseProfile(r io.Reader) (map[string]*fileCoverage, error) {
	files := make(map[string]*fileCoverage)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "mode:") {
			continue
		}
		if err := addBlock(files, text); err != nil {
			return nil, fmt.Errorf("profile line %d: %w", line, err)
		}
	}
	return files, scanner.Err()
}

func addBlock(files map[string]*fileCoverage, text string) error {
	fields := strings.Fields(text)
	if len(fields) != 3 {
		return fmt.Errorf("expected 3 fields, got %d", len(fields))
	}
	name, span, ok := strings.Cut(fields[0], ":")
	if !ok {
		return fmt.Errorf("missing range in %q", fields[0])
	}
	stmts, err := strconv.Atoi(fields[1])
	if err != nil {
		return fmt.Errorf("statements: %w", err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return fmt.Errorf("count: %w", err)
	}
	fc := files[name]
	if fc == nil {
		fc = &fileCoverage{blocks: make(map[string]block)}
		files[name] = fc
	}
	b := fc.blocks[span]
	b.stmts = stmts
	b.covered = b.covered || count > 0
	fc.blocks[span] = b
	return nil
}
