package render_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

var formatCases = []struct {
	in   string
	want render.Format
	ok   bool
}{
	{"table", render.FormatTable, true},
	{" MD ", render.FormatMarkdown, true},
	{"compact", render.FormatCompact, true},
	{"json", render.FormatJSON, true},
	{"yaml", "", false},
}

func TestParseFormat(t *testing.T) {
	for _, tc := range formatCases {
		got, err := render.ParseFormat(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("%q: got %q, %v", tc.in, got, err)
		}
		if !tc.ok && !errors.Is(err, domain.ErrUsage) {
			t.Errorf("%q: expected ErrUsage, got %v", tc.in, err)
		}
	}
	if len(render.Formats) != 4 {
		t.Fatalf("Formats = %v", render.Formats)
	}
}

func TestRenderUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := render.Render(&buf, render.NewDocument("x"), render.Format("yaml"))
	if !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("nothing must be written on a usage error, got %q", buf.String())
	}
}

var emptyCases = []struct {
	format render.Format
	want   string
}{
	{render.FormatTable, ""},
	{render.FormatCompact, ""},
	{render.FormatMarkdown, ""},
	{render.FormatJSON, "{\"sections\":[]}\n"},
}

func TestRenderEmptyAndNil(t *testing.T) {
	for _, tc := range emptyCases {
		for _, doc := range []*render.Document{nil, render.NewDocument("")} {
			var buf bytes.Buffer
			if err := render.Render(&buf, doc, tc.format); err != nil {
				t.Fatalf("%s: %v", tc.format, err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("%s: got %q, want %q", tc.format, got, tc.want)
			}
		}
	}
}

func TestRenderTitledEmptyPrintsOnlyTheTitle(t *testing.T) {
	for _, format := range render.Formats[:3] {
		var buf bytes.Buffer
		if err := render.Render(&buf, titledEmpty(), format); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if got := buf.String(); !strings.Contains(got, "Board") || strings.Contains(got, "Empty") {
			t.Errorf("%s: got %q", format, got)
		}
	}
}

// titledEmpty has a title and sections without content: only the title
// shows in text formats.
func titledEmpty() *render.Document {
	doc := render.NewDocument("Board")
	doc.AddSection("Empty")
	doc.AddSection("").SetTable("A", "B")
	return doc
}

// failingWriter fails once the limit of bytes has been written, to reach
// the error paths of every renderer.
type failingWriter struct {
	limit   int
	written int
}

var errWrite = errors.New("disk full")

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.written+len(p) > f.limit {
		return 0, errWrite
	}
	f.written += len(p)
	return len(p), nil
}

func TestRenderWriteErrors(t *testing.T) {
	for _, format := range render.Formats {
		for _, limit := range []int{0, 12, 40} {
			err := render.Render(&failingWriter{limit: limit}, fixtureDocument(), format)
			if !errors.Is(err, errWrite) {
				t.Errorf("%s with limit %d: expected the write error, got %v", format, limit, err)
			}
		}
	}
}

func TestRenderNeverPrintsEscapes(t *testing.T) {
	doc := render.NewDocument("\x1b[1mTitle\x1b[0m")
	s := doc.AddSection("\x1b]0;evil\x07Section")
	s.AddKeyValue("\x1b[31mName", "va\x00lue").AddItem("it\x07em").AddNote("no\x1b[Kte")
	s.SetTable("\x1b[2JA", "B").AddRow("\x1b[31mred", "x\x7fy")
	for _, format := range render.Formats {
		var buf bytes.Buffer
		if err := render.Render(&buf, doc, format); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		for _, c := range buf.String() {
			if c < 0x20 && c != '\n' && c != '\t' {
				t.Fatalf("%s printed control character %q: %q", format, c, buf.String())
			}
		}
	}
}

func TestRenderWidthOnlyAffectsTable(t *testing.T) {
	narrow := render.Options{Width: 30}
	for _, format := range render.Formats[1:] {
		var wide, fitted bytes.Buffer
		if err := render.Render(&wide, fixtureDocument(), format); err != nil {
			t.Fatal(err)
		}
		if err := narrow.Render(&fitted, fixtureDocument(), format); err != nil {
			t.Fatal(err)
		}
		if wide.String() != fitted.String() {
			t.Errorf("%s must ignore the width", format)
		}
	}
	if err := narrow.Render(io.Discard, nil, render.FormatTable); err != nil {
		t.Fatal(err)
	}
}
