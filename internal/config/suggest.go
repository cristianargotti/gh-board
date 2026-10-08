package config

import (
	"strings"
	"unicode/utf8"
)

// maxSuggestDistance is the edit distance under which a name counts as a
// typo of a known one.
const maxSuggestDistance = 2

// withHint appends a "did you mean" to a reason when a known name is close
// to the one in the file: same letters in another case, or at most two
// edits away.
func withHint(reason, name string, known []string) string {
	if hint := suggest(name, known); hint != "" {
		return reason + "; did you mean " + hint + "?"
	}
	return reason
}

// suggest returns the closest known name, or "" when none is close.
func suggest(name string, known []string) string {
	for _, k := range known {
		if strings.EqualFold(strings.TrimSpace(k), strings.TrimSpace(name)) {
			return quoteName(k)
		}
	}
	if utf8.RuneCountInString(name) <= maxSuggestDistance {
		return ""
	}
	best, bestDistance := "", maxSuggestDistance+1
	for _, k := range known {
		if d := distance(strings.ToLower(name), strings.ToLower(k)); d < bestDistance {
			best, bestDistance = k, d
		}
	}
	if best == "" {
		return ""
	}
	return quoteName(best)
}

func quoteName(s string) string {
	return `"` + s + `"`
}

// distance is the Levenshtein distance over runes, with a single row of
// state because the names are short.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	row := make([]int, len(rb)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		prev := row[0]
		row[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			current := row[j]
			row[j] = minInt(row[j]+1, row[j-1]+1, prev+cost)
			prev = current
		}
	}
	return row[len(rb)]
}

func minInt(values ...int) int {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
