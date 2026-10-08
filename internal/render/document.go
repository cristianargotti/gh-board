// Package render turns command results into table, compact, markdown and
// JSON output and sanitizes untrusted text before it reaches a terminal or
// an agent (section 6.8).
package render

import "unicode/utf8"

// Document is the renderer input: a title, ordered sections and, for
// --json, the structured result the command produced.
type Document struct {
	Title    string    `json:"title,omitempty"`
	Sections []Section `json:"sections"`
	// Data is the structured form for --json; when set, the JSON renderer
	// encodes it instead of the document.
	Data any `json:"-"`
}

// Section groups key values, a table, a list and notes under a title. Any
// member may be empty; renderers skip what is absent.
type Section struct {
	Title     string     `json:"title,omitempty"`
	KeyValues []KeyValue `json:"key_values,omitempty"`
	Table     *Table     `json:"table,omitempty"`
	List      []string   `json:"list,omitempty"`
	Notes     []string   `json:"notes,omitempty"`
}

// KeyValue is one labeled value.
type KeyValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Table is a grid with a header row.
type Table struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

// NewDocument starts a document.
func NewDocument(title string) *Document {
	return &Document{Title: title}
}

// AddSection appends a section and returns it for chaining.
func (d *Document) AddSection(title string) *Section {
	d.Sections = append(d.Sections, Section{Title: title})
	return &d.Sections[len(d.Sections)-1]
}

// IsEmpty reports whether no section carries content.
func (d *Document) IsEmpty() bool {
	for _, s := range d.Sections {
		if !s.IsEmpty() {
			return false
		}
	}
	return true
}

// AddKeyValue appends a labeled value.
func (s *Section) AddKeyValue(name, value string) *Section {
	s.KeyValues = append(s.KeyValues, KeyValue{Name: name, Value: value})
	return s
}

// SetTable creates the section table with the given columns.
func (s *Section) SetTable(columns ...string) *Table {
	s.Table = &Table{Columns: columns}
	return s.Table
}

// AddItem appends a list entry.
func (s *Section) AddItem(text string) *Section {
	s.List = append(s.List, text)
	return s
}

// AddNote appends a note shown after the section content.
func (s *Section) AddNote(text string) *Section {
	s.Notes = append(s.Notes, text)
	return s
}

// IsEmpty reports whether the section has no content besides its title.
func (s Section) IsEmpty() bool {
	return len(s.KeyValues) == 0 && len(s.List) == 0 && len(s.Notes) == 0 &&
		(s.Table == nil || len(s.Table.Rows) == 0)
}

// AddRow appends a row, padded or cut to the number of columns.
func (t *Table) AddRow(cells ...string) *Table {
	row := make([]string, len(t.Columns))
	copy(row, cells)
	t.Rows = append(t.Rows, row)
	return t
}

// Width returns the widest cell of a column, header included, in runes.
func (t *Table) Width(column int) int {
	if column < 0 || column >= len(t.Columns) {
		return 0
	}
	width := utf8.RuneCountInString(t.Columns[column])
	for _, row := range t.Rows {
		if column >= len(row) {
			continue
		}
		if w := utf8.RuneCountInString(row[column]); w > width {
			width = w
		}
	}
	return width
}
