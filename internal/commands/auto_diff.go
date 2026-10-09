package commands

import (
	"bytes"
	"fmt"
	"strings"
)

// autoDiffContext is the number of unchanged lines shown around a change.
const autoDiffContext = 3

// autoDiffLimit bounds the product of the line counts compared in detail;
// beyond it the diff shows the whole file replaced instead of allocating
// a huge table.
const autoDiffLimit = 4_000_000

// autoDiffOp is one line of a diff: ' ' unchanged, '-' removed, '+' added.
type autoDiffOp struct {
	kind byte
	text string
}

// autoDiffHunk is a range of ops printed together.
type autoDiffHunk struct {
	start, end int
}

// autoDiff renders a unified diff between two versions of a file, or ""
// when they are equal. The installers print it before writing.
func autoDiff(path string, before, after []byte) string {
	if bytes.Equal(before, after) {
		return ""
	}
	ops := autoDiffOps(autoSplitLines(before), autoSplitLines(after))
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "--- %s\n+++ %s\n", path, path)
	oldPos, newPos, cursor := 1, 1, 0
	for _, h := range autoDiffHunks(ops) {
		for ; cursor < h.start; cursor++ {
			if ops[cursor].kind != '+' {
				oldPos++
			}
			if ops[cursor].kind != '-' {
				newPos++
			}
		}
		autoWriteHunk(&sb, ops, h, oldPos, newPos)
	}
	return sb.String()
}

// autoWriteHunk prints one hunk with its unified header.
func autoWriteHunk(sb *strings.Builder, ops []autoDiffOp, h autoDiffHunk, oldStart, newStart int) {
	oldCount, newCount := 0, 0
	for _, op := range ops[h.start:h.end] {
		if op.kind != '+' {
			oldCount++
		}
		if op.kind != '-' {
			newCount++
		}
	}
	_, _ = fmt.Fprintf(sb, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for _, op := range ops[h.start:h.end] {
		sb.WriteByte(op.kind)
		sb.WriteString(op.text)
		sb.WriteByte('\n')
	}
}

// autoDiffHunks groups the changes with their context, merging hunks whose
// context would overlap.
func autoDiffHunks(ops []autoDiffOp) []autoDiffHunk {
	var changes []int
	for i, op := range ops {
		if op.kind != ' ' {
			changes = append(changes, i)
		}
	}
	var hunks []autoDiffHunk
	for i := 0; i < len(changes); {
		first, last := changes[i], changes[i]
		for i++; i < len(changes) && changes[i]-last <= 2*autoDiffContext+1; i++ {
			last = changes[i]
		}
		hunks = append(hunks, autoDiffHunk{
			start: max(first-autoDiffContext, 0),
			end:   min(last+autoDiffContext+1, len(ops)),
		})
	}
	return hunks
}

// autoDiffOps aligns two line lists on their longest common subsequence.
func autoDiffOps(a, b []string) []autoDiffOp {
	table := autoLCSTable(a, b)
	if table == nil {
		return autoReplaceOps(a, b)
	}
	var ops []autoDiffOp
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, autoDiffOp{' ', a[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, autoDiffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, autoDiffOp{'+', b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, autoDiffOp{'-', a[i]})
	}
	for ; j < len(b); j++ {
		ops = append(ops, autoDiffOp{'+', b[j]})
	}
	return ops
}

// autoLCSTable fills the suffix table of the longest common subsequence,
// or returns nil when the line counts exceed autoDiffLimit; the bounds are
// checked here, next to the allocations they protect.
func autoLCSTable(a, b []string) [][]int {
	if len(a) > autoDiffLimit || len(b) > autoDiffLimit || len(a)*len(b) > autoDiffLimit {
		return nil
	}
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table
}

// autoReplaceOps shows every old line removed and every new line added.
func autoReplaceOps(a, b []string) []autoDiffOp {
	var ops []autoDiffOp
	for _, line := range a {
		ops = append(ops, autoDiffOp{'-', line})
	}
	for _, line := range b {
		ops = append(ops, autoDiffOp{'+', line})
	}
	return ops
}

// autoSplitLines splits file content into lines without the final newline.
func autoSplitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}
