package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Format selects the output renderer.
type Format string

// Output formats of the --format flag; --json selects FormatJSON.
const (
	FormatTable    Format = "table"
	FormatCompact  Format = "compact"
	FormatMarkdown Format = "md"
	FormatJSON     Format = "json"
)

// Formats lists the accepted formats for help text and validation.
var Formats = []Format{FormatTable, FormatCompact, FormatMarkdown, FormatJSON}

// ParseFormat validates a --format value.
func ParseFormat(s string) (Format, error) {
	for _, f := range Formats {
		if string(f) == strings.ToLower(strings.TrimSpace(s)) {
			return f, nil
		}
	}
	return "", fmt.Errorf("format %q: expected table, compact, md or json: %w", s, domain.ErrUsage)
}

// Options tunes the terminal renderers.
type Options struct {
	// Width is the number of terminal columns available to the table
	// format. Columns shrink, widest first, until every row fits one line.
	// Zero or less means no limit; the caller measures the terminal.
	Width int
}

// Render writes the document in the format with default options. Table and
// compact are for the terminal, md for chat and documents, json for agents
// and scripts.
func Render(w io.Writer, doc *Document, format Format) error {
	return Options{}.Render(w, doc, format)
}

// Render writes the document in the format with these options. Every string
// of the document is sanitized first, so no renderer prints raw ANSI or
// control characters whatever the command passed in. The table format,
// which is for people, also drops the data delimiters around external
// text; compact, md and json keep them for the agents (section 6.8). The
// JSON payload is encoded verbatim: JSON escaping already neutralizes it
// for a terminal.
func (o Options) Render(w io.Writer, doc *Document, format Format) error {
	if doc == nil {
		doc = NewDocument("")
	}
	clean := doc.sanitized()
	switch format {
	case FormatTable:
		return renderTable(w, clean.undelimited(), o.Width)
	case FormatCompact:
		return renderCompact(w, clean)
	case FormatMarkdown:
		return renderMarkdown(w, clean)
	case FormatJSON:
		return renderJSON(w, clean)
	}
	return fmt.Errorf("format %q: expected table, compact, md or json: %w", format, domain.ErrUsage)
}
