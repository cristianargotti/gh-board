package render

// sanitized returns a copy of the document with every string cleaned for a
// terminal and every table row normalized to its columns. The copy keeps
// the JSON payload untouched.
func (d *Document) sanitized() *Document {
	return d.mapped(func(s string) string { return Sanitize(s, 0) })
}

// mapped returns a copy of the document with fn applied to every string
// and every table row normalized to its columns. The copy keeps the JSON
// payload untouched.
func (d *Document) mapped(fn func(string) string) *Document {
	out := &Document{
		Title:    fn(d.Title),
		Sections: make([]Section, 0, len(d.Sections)),
		Data:     d.Data,
	}
	for _, s := range d.Sections {
		out.Sections = append(out.Sections, s.mapped(fn))
	}
	return out
}

func (s Section) mapped(fn func(string) string) Section {
	out := Section{
		Title: fn(s.Title),
		List:  mapAll(s.List, fn),
		Notes: mapAll(s.Notes, fn),
	}
	for _, kv := range s.KeyValues {
		out.KeyValues = append(out.KeyValues, KeyValue{Name: fn(kv.Name), Value: fn(kv.Value)})
	}
	if s.Table != nil {
		out.Table = s.Table.mapped(fn)
	}
	return out
}

func (t *Table) mapped(fn func(string) string) *Table {
	out := &Table{
		Columns: mapAll(t.Columns, fn),
		Rows:    make([][]string, 0, len(t.Rows)),
	}
	for _, row := range t.Rows {
		cells := make([]string, len(t.Columns))
		for i := range cells {
			if i < len(row) {
				cells[i] = fn(row[i])
			}
		}
		out.Rows = append(out.Rows, cells)
	}
	return out
}

func mapAll(list []string, fn func(string) string) []string {
	if len(list) == 0 {
		return nil
	}
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = fn(s)
	}
	return out
}
