package render

import (
	"io"
	"strings"
)

// renderCompact writes one line per row for agents and scripts: markdown
// headings for the title and sections, "name: value" lines, tab separated
// grid rows with the header first, "- " list entries and "> " notes. No
// alignment, no blank lines. Sanitized cells never contain a tab, so the
// separator is unambiguous.
func renderCompact(w io.Writer, doc *Document) error {
	p := &printer{w: w}
	if doc.Title != "" {
		p.line("# ", doc.Title)
	}
	for _, s := range doc.Sections {
		if !s.IsEmpty() {
			writeCompactSection(p, s)
		}
	}
	return p.err
}

func writeCompactSection(p *printer, s Section) {
	if s.Title != "" {
		p.line("## ", s.Title)
	}
	for _, kv := range s.KeyValues {
		p.line(strings.TrimRight(kv.Name+": "+kv.Value, " "))
	}
	if s.Table != nil && len(s.Table.Rows) > 0 {
		p.line(strings.Join(s.Table.Columns, "\t"))
		for _, row := range s.Table.Rows {
			p.line(strings.Join(row, "\t"))
		}
	}
	for _, item := range s.List {
		p.line("- ", item)
	}
	for _, note := range s.Notes {
		p.line("> ", note)
	}
}
