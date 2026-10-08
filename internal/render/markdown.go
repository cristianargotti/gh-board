package render

import (
	"io"
	"strings"
)

// renderMarkdown writes GitHub flavored markdown: headings, bold labels,
// pipe tables with the pipe escaped inside cells, list entries and
// blockquoted notes, every block separated by a blank line.
func renderMarkdown(w io.Writer, doc *Document) error {
	p := &printer{w: w}
	b := &blocks{p: p}
	if doc.Title != "" {
		b.start()
		p.line("# ", doc.Title)
	}
	for _, s := range doc.Sections {
		if !s.IsEmpty() {
			writeMarkdownSection(p, b, s)
		}
	}
	return p.err
}

func writeMarkdownSection(p *printer, b *blocks, s Section) {
	if s.Title != "" {
		b.start()
		p.line("## ", s.Title)
	}
	if len(s.KeyValues) > 0 {
		b.start()
		for _, kv := range s.KeyValues {
			p.line(strings.TrimRight("- **"+kv.Name+"**: "+kv.Value, " "))
		}
	}
	if s.Table != nil && len(s.Table.Rows) > 0 {
		b.start()
		writeMarkdownTable(p, s.Table)
	}
	if len(s.List) > 0 {
		b.start()
		for _, item := range s.List {
			p.line("- ", item)
		}
	}
	if len(s.Notes) > 0 {
		b.start()
		for _, note := range s.Notes {
			p.line("> ", note)
		}
	}
}

func writeMarkdownTable(p *printer, t *Table) {
	p.line(markdownRow(t.Columns))
	rule := make([]string, len(t.Columns))
	for i := range rule {
		rule[i] = "---"
	}
	p.line(markdownRow(rule))
	for _, row := range t.Rows {
		p.line(markdownRow(row))
	}
}

// markdownRow joins the cells as a pipe table row; a pipe inside a cell is
// escaped so untrusted text cannot add columns.
func markdownRow(cells []string) string {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = strings.ReplaceAll(cell, "|", `\|`)
	}
	return "| " + strings.Join(escaped, " | ") + " |"
}
