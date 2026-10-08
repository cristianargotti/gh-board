package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/render"
)

func TestJSONPrefersPayload(t *testing.T) {
	doc := render.NewDocument("ignored")
	doc.AddSection("ignored").AddKeyValue("a", "b")
	doc.Data = map[string]any{"z": 1, "a": []string{"<x>", "y&z"}, "esc": "a\x1bb"}
	var buf bytes.Buffer
	if err := render.Render(&buf, doc, render.FormatJSON); err != nil {
		t.Fatal(err)
	}
	want := "{\"a\":[\"<x>\",\"y&z\"],\"esc\":\"a\\u001bb\",\"z\":1}\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestJSONDocumentForm(t *testing.T) {
	doc := render.NewDocument("Board")
	s := doc.AddSection("Items")
	s.SetTable("REF", "TITLE").AddRow("#1", "x\x1b[0my")
	s.AddKeyValue("k", "v").AddItem("i").AddNote("n")
	doc.AddSection("Empty")
	var buf bytes.Buffer
	if err := render.Render(&buf, doc, render.FormatJSON); err != nil {
		t.Fatal(err)
	}
	var back render.Document
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("output is not JSON: %v: %s", err, buf.String())
	}
	if back.Title != "Board" || len(back.Sections) != 2 || back.Sections[0].Table.Rows[0][1] != "xy" {
		t.Fatalf("round trip lost content: %+v", back)
	}
	if strings.Count(buf.String(), "\n") != 1 || !strings.HasSuffix(buf.String(), "}\n") {
		t.Fatalf("expected one line with a trailing newline: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "\"sections\":[{") {
		t.Fatalf("sections must encode as an array: %q", buf.String())
	}
}

func TestJSONPayloadError(t *testing.T) {
	doc := render.NewDocument("")
	doc.Data = make(chan int)
	err := render.Render(&bytes.Buffer{}, doc, render.FormatJSON)
	if err == nil || !strings.Contains(err.Error(), "render json") {
		t.Fatalf("expected an encoding error, got %v", err)
	}
}
