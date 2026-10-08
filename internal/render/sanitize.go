package render

import (
	"github.com/cristianargotti/gh-board/internal/domain"
)

// Limits of section 6.8, re-exported so a command building a document needs
// only this package for presentation concerns.
const (
	// TitleLimit is the maximum length of an issue title in runes.
	TitleLimit = domain.TitleLimit
	// ExcerptLimit is the maximum length of a body excerpt in runes.
	ExcerptLimit = domain.ExcerptLimit
	// Ellipsis marks a truncated text.
	Ellipsis = domain.Ellipsis
)

// Labels of the data delimiters commands use for external text.
const (
	// LabelTitle marks an issue title.
	LabelTitle = "title"
	// LabelBody marks an issue body excerpt.
	LabelBody = "body"
	// LabelComment marks a comment excerpt.
	LabelComment = "comment"
	// LabelReadme marks project README content.
	LabelReadme = "readme"
	// LabelValue marks a free text value of a board field.
	LabelValue = "value"
)

// Grammar of domain.Delimit, which the table format recognizes to drop
// the tags around external text. TestDelimiterGrammarMatchesDomain pins it.
const (
	dataOpen  = "[[begin:"
	dataClose = "[[end:"
	dataEnd   = "]]"
)

// Sanitize prepares untrusted text for a terminal or a prompt through the
// domain sanitizer: ANSI escape sequences, control characters, invisible
// format characters and the replacement character are removed, whitespace
// (newlines included) collapses to single spaces, the result is trimmed and
// cut to limit runes with an ellipsis. A limit of zero or less keeps the
// whole text.
func Sanitize(text string, limit int) string {
	return domain.CleanText(text, limit)
}

// Delimit cleans external text to limit runes and wraps it in the domain's
// data delimiters on one line, the only form a cell, a value, a list entry
// or a note can hold: "[[begin:label]] text [[end:label]]". The domain
// breaks bracket pairs inside the text, so it can never forge a tag.
func Delimit(label, text string, limit int) string {
	return Sanitize(domain.Delimit(label, Sanitize(text, limit)), 0)
}

// Title delimits an issue title cut to TitleLimit.
func Title(text string) string {
	return Delimit(LabelTitle, text, TitleLimit)
}

// Excerpt delimits a body, comment or README excerpt cut to ExcerptLimit.
func Excerpt(label, text string) string {
	return Delimit(label, text, ExcerptLimit)
}
