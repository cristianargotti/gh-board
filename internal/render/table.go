package render

import (
	"io"
	"strings"
	"unicode/utf8"
)

const (
	// indent leads every content line under a section title.
	indent = "  "
	// gap separates the columns of a grid.
	gap = "  "
)

// renderTable writes the terminal format: aligned grids, labeled values
// padded to one column, blank lines between sections and blocks. width is
// the terminal width the grids must fit, zero or less for no limit.
func renderTable(w io.Writer, doc *Document, width int) error {
	p := &printer{w: w}
	b := &blocks{p: p}
	if doc.Title != "" {
		b.start()
		p.line(doc.Title)
	}
	for _, s := range doc.Sections {
		if s.IsEmpty() {
			continue
		}
		b.start()
		writeTableSection(p, s, width-len(indent))
	}
	return p.err
}

// writeTableSection writes the title and then each block of the section,
// blocks separated by a blank line.
func writeTableSection(p *printer, s Section, available int) {
	if s.Title != "" {
		p.line(s.Title)
	}
	b := &blocks{p: p}
	if len(s.KeyValues) > 0 {
		b.start()
		writeKeyValues(p, s.KeyValues)
	}
	if s.Table != nil && len(s.Table.Rows) > 0 {
		b.start()
		writeGrid(p, s.Table, available)
	}
	if len(s.List) > 0 {
		b.start()
		for _, item := range s.List {
			p.line(indent, "- ", item)
		}
	}
	if len(s.Notes) > 0 {
		b.start()
		for _, note := range s.Notes {
			p.line(indent, note)
		}
	}
}

// writeKeyValues aligns the values on one column after the longest name.
func writeKeyValues(p *printer, kvs []KeyValue) {
	width := 0
	for _, kv := range kvs {
		if n := utf8.RuneCountInString(kv.Name); n > width {
			width = n
		}
	}
	for _, kv := range kvs {
		label := pad(kv.Name+":", width+1)
		p.line(indent, strings.TrimRight(label+gap+kv.Value, " "))
	}
}

// writeGrid writes the header and the rows with every column padded to the
// fitted width; the last column is never padded, so lines carry no
// trailing spaces.
func writeGrid(p *printer, t *Table, available int) {
	widths := fitWidths(naturalWidths(t), available)
	p.line(indent, joinCells(t.Columns, widths))
	for _, row := range t.Rows {
		p.line(indent, joinCells(row, widths))
	}
}

func joinCells(cells []string, widths []int) string {
	var b strings.Builder
	for i, cell := range cells {
		if i > 0 {
			b.WriteString(gap)
		}
		b.WriteString(pad(truncateCell(cell, widths[i]), widths[i]))
	}
	return strings.TrimRight(b.String(), " ")
}
