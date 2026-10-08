package render_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/render"
)

func TestDocumentModel(t *testing.T) {
	doc := render.NewDocument("Board")
	if doc.Title != "Board" || !doc.IsEmpty() {
		t.Fatal("a new document is empty")
	}
	sec := doc.AddSection("Sprint")
	if sec.Title != "Sprint" || !sec.IsEmpty() || !doc.IsEmpty() {
		t.Fatal("a titled section without content is empty")
	}
	sec.AddKeyValue("Dias restantes", "3").AddItem("item").AddNote("nota")
	if len(sec.KeyValues) != 1 || sec.KeyValues[0].Name != "Dias restantes" || sec.KeyValues[0].Value != "3" {
		t.Fatalf("KeyValues = %+v", sec.KeyValues)
	}
	if len(sec.List) != 1 || len(sec.Notes) != 1 || sec.IsEmpty() || doc.IsEmpty() {
		t.Fatal("content must make the section and document non-empty")
	}
	if len(doc.Sections) != 1 || doc.Sections[0].List[0] != "item" {
		t.Fatal("AddSection must return a pointer into the document")
	}
}

func TestTable(t *testing.T) {
	doc := render.NewDocument("")
	sec := doc.AddSection("Itens")
	tbl := sec.SetTable("Ref", "Status")
	if sec.IsEmpty() == false {
		t.Fatal("an empty table does not count as content")
	}
	tbl.AddRow("#12", "DONE", "extra").AddRow("#1234")
	if len(tbl.Rows) != 2 || len(tbl.Rows[0]) != 2 || tbl.Rows[0][1] != "DONE" {
		t.Fatalf("rows must be cut to the columns: %v", tbl.Rows)
	}
	if tbl.Rows[1][1] != "" {
		t.Fatal("short rows must be padded")
	}
	if sec.IsEmpty() || doc.IsEmpty() {
		t.Fatal("rows make the table content")
	}
	if w := tbl.Width(0); w != 5 {
		t.Fatalf("Width(0) = %d", w)
	}
	if w := tbl.Width(1); w != 6 {
		t.Fatalf("Width(1) = %d", w)
	}
	if tbl.Width(2) != 0 || tbl.Width(-1) != 0 {
		t.Fatal("out of range columns have zero width")
	}
	tbl.AddRow("Épico", "x")
	if w := tbl.Width(0); w != 5 {
		t.Fatalf("Width counts runes, got %d", w)
	}
}

func TestTableWidthWithRaggedRows(t *testing.T) {
	tbl := &render.Table{Columns: []string{"A", "B"}, Rows: [][]string{{"abcd"}, {"x", "yz"}}}
	if w := tbl.Width(1); w != 2 {
		t.Fatalf("Width(1) = %d, want 2 (short rows skipped)", w)
	}
	if w := tbl.Width(0); w != 4 {
		t.Fatalf("Width(0) = %d", w)
	}
}
