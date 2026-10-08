package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Limits of section 6.8 for untrusted text.
const (
	// TitleLimit is the maximum length of an issue title in runes.
	TitleLimit = 120
	// ExcerptLimit is the maximum length of a body excerpt in runes.
	ExcerptLimit = 280
	// Ellipsis marks a truncated text.
	Ellipsis = "..."
)

// ansiPattern matches CSI sequences, OSC sequences terminated by BEL or
// ST, and any other two-byte escape.
var ansiPattern = regexp.MustCompile(
	`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b.`,
)

// CleanText strips ANSI sequences, control characters, invisible format
// characters and the replacement character, collapses whitespace and
// truncates to limit runes with an ellipsis. A limit of zero or less keeps
// the whole text.
func CleanText(text string, limit int) string {
	clean, _ := Excerpt(text, limit)
	return clean
}

// Excerpt cleans the text like CleanText and also reports how many runes
// the truncation dropped.
func Excerpt(text string, limit int) (string, int) {
	stripped := ansiPattern.ReplaceAllString(text, "")
	var b strings.Builder
	b.Grow(len(stripped))
	space := false
	for _, r := range stripped {
		switch {
		case unicode.IsSpace(r):
			space = true
		case unicode.IsControl(r), r == unicode.ReplacementChar, unicode.Is(unicode.Cf, r):
			continue
		default:
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		}
	}
	return Truncate(b.String(), limit)
}

// Truncate cuts the text to limit runes, ending it with the ellipsis, and
// returns the number of runes dropped.
func Truncate(s string, limit int) (string, int) {
	if limit <= 0 {
		return s, 0
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s, 0
	}
	keep := limit - len(Ellipsis)
	if keep < 0 {
		keep = 0
	}
	kept := strings.TrimRight(string(runes[:keep]), " ")
	return kept + Ellipsis, len(runes) - keep
}

// CleanTitle cleans an issue title to the title limit.
func CleanTitle(s string) string { return CleanText(s, TitleLimit) }

// CleanBody cleans an issue body or comment to the excerpt limit.
func CleanBody(s string) string { return CleanText(s, ExcerptLimit) }

// Data delimiters wrap every external text so a reader, human or agent,
// sees where the data starts and ends (F6).
const (
	dataOpen  = "[[begin:"
	dataClose = "[[end:"
	dataEnd   = "]]"
)

// Delimit wraps already cleaned external text in explicit data delimiters
// named by label. Bracket pairs inside the text are broken so the text can
// never close or open a delimiter itself.
func Delimit(label, text string) string {
	safe := strings.NewReplacer("[[", "[ [", "]]", "] ]").Replace(text)
	return dataOpen + label + dataEnd + "\n" + safe + "\n" + dataClose + label + dataEnd
}

// OmittedNote states how many entries a listing left out, or "" for none.
func OmittedNote(n int) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return "1 item omitted"
	default:
		return fmt.Sprintf("%d items omitted", n)
	}
}
