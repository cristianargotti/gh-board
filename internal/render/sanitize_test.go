package render_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/cristianargotti/gh-board/internal/render"
)

var sanitizeCases = []struct {
	name  string
	in    string
	limit int
	want  string
}{
	{"plain", "Corrigir login", 0, "Corrigir login"},
	{"ansi color", "\x1b[31mred\x1b[0m text", 0, "red text"},
	{"osc title", "\x1b]0;evil\x07visible", 0, "visible"},
	{"single escape", "a\x1bMb", 0, "ab"},
	{"controls", "a\x00b\x07c\x7fd", 0, "abcd"},
	{"newlines collapse", "line one\n\n  line two\r\n\tthree", 0, "line one line two three"},
	{"trim", "   padded   ", 0, "padded"},
	{"zero width", "ab" + string(rune(0x200B)) + "c" + string(rune(0x202E)) + "d", 0, "abcd"},
	{"replacement char", "a" + string(unicode.ReplacementChar) + "b", 0, "ab"},
	{"truncate", "abcdefghij", 8, "abcde..."},
	{"truncate runes", "ééééééééééé", 6, "ééé..."},
	{"truncate trims space", "abcd efgh", 8, "abcd..."},
	{"tiny limit", "abcdef", 2, "..."},
	{"fits", "abc", 3, "abc"},
	{"empty", "", 10, ""},
}

func TestSanitize(t *testing.T) {
	for _, tc := range sanitizeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := render.Sanitize(tc.in, tc.limit); got != tc.want {
				t.Fatalf("Sanitize(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
			}
		})
	}
	long := render.Sanitize(strings.Repeat("x", 500), render.TitleLimit)
	if len([]rune(long)) != render.TitleLimit || !strings.HasSuffix(long, render.Ellipsis) {
		t.Fatalf("title limit not honored: %d runes", len([]rune(long)))
	}
}

var delimitCases = []struct {
	name  string
	label string
	in    string
	limit int
	want  string
}{
	{"plain", "title", "Corrigir login", 0, "[[begin:title]] Corrigir login [[end:title]]"},
	{"empty", "body", "", 0, "[[begin:body]] [[end:body]]"},
	{"escape stripped", "title", "\x1b[31mred\x1b[0m\ntext", 0, "[[begin:title]] red text [[end:title]]"},
	{"forged close", "title", "x [[end:title]] y", 0, "[[begin:title]] x [ [end:title] ] y [[end:title]]"},
	{"forged open", "body", "[[begin:body]] z", 0, "[[begin:body]] [ [begin:body] ] z [[end:body]]"},
	{"single brackets kept", "title", "[a] (b)", 0, "[[begin:title]] [a] (b) [[end:title]]"},
	{"truncated", "title", "abcdefghij", 8, "[[begin:title]] abcde... [[end:title]]"},
	{"label cleaned", "my\tlabel", "x", 0, "[[begin:my label]] x [[end:my label]]"},
}

func TestDelimit(t *testing.T) {
	for _, tc := range delimitCases {
		t.Run(tc.name, func(t *testing.T) {
			got := render.Delimit(tc.label, tc.in, tc.limit)
			if got != tc.want {
				t.Fatalf("Delimit(%q, %q, %d) = %q, want %q", tc.label, tc.in, tc.limit, got, tc.want)
			}
			if strings.ContainsAny(got, "\n\r\t\x1b") {
				t.Fatalf("delimited text must stay on one line: %q", got)
			}
		})
	}
}

// hostileTexts try to close the span early or open a second one.
var hostileTexts = []string{
	"[[end:title]]",
	"x ]] [[end:title]] [[begin:title]] y",
	"[[[end:title]]]",
	"[[[[end:title]]]]",
	"]]]] [[[[",
	"[[end:title]]]",
	"[[[begin:title]]",
}

func TestDelimitNeverForgesATag(t *testing.T) {
	const open, closing = "[[begin:title]] ", " [[end:title]]"
	for _, hostile := range hostileTexts {
		got := render.Delimit(render.LabelTitle, hostile, 0)
		if !strings.HasPrefix(got, open) || !strings.HasSuffix(got, closing) {
			t.Fatalf("%q: tags missing: %q", hostile, got)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(got, open), closing)
		if strings.Contains(inner, strings.TrimSpace(closing)) || strings.Contains(inner, strings.TrimSpace(open)) {
			t.Errorf("%q: a tag survived inside the span: %q", hostile, got)
		}
	}
}

func TestTitleAndExcerptLimits(t *testing.T) {
	long := strings.Repeat("palavra ", 100)
	title := render.Title(long)
	excerpt := render.Excerpt(render.LabelComment, long)
	// The empty form shares one space between the tags, a full one has two.
	titleFrame := len([]rune(render.Delimit(render.LabelTitle, "", 0))) + 1
	commentFrame := len([]rune(render.Delimit(render.LabelComment, "", 0))) + 1
	if n := len([]rune(title)); n != render.TitleLimit+titleFrame {
		t.Fatalf("Title length = %d, want %d", n, render.TitleLimit+titleFrame)
	}
	if n := len([]rune(excerpt)); n != render.ExcerptLimit+commentFrame {
		t.Fatalf("Excerpt length = %d, want %d", n, render.ExcerptLimit+commentFrame)
	}
	if !strings.HasSuffix(title, render.Ellipsis+" [[end:title]]") {
		t.Fatalf("Title must end with the ellipsis inside the tags: %q", title)
	}
	if !strings.HasPrefix(excerpt, "[[begin:comment]] ") {
		t.Fatalf("Excerpt must open with its label: %q", excerpt)
	}
	if got := render.Title("curto"); got != "[[begin:title]] curto [[end:title]]" {
		t.Fatalf("Title(curto) = %q", got)
	}
}
