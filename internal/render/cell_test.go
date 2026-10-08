package render

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// The one-line form relies on the exact tags the domain writes; a change
// there must reach the constants of this package.
func TestDelimiterGrammarMatchesDomain(t *testing.T) {
	want := dataOpen + "x" + dataEnd + "\n" + "y" + "\n" + dataClose + "x" + dataEnd
	if got := domain.Delimit("x", "y"); got != want {
		t.Fatalf("domain.Delimit grammar changed: got %q, want %q", got, want)
	}
}

var undelimitCases = []struct {
	name string
	in   string
	want string
}{
	{"plain", "plain text", "plain text"},
	{"one span", "[[begin:title]] text [[end:title]]", "text"},
	{"empty span", "[[begin:title]] [[end:title]]", ""},
	{"empty two spaces", "[[begin:title]]  [[end:title]]", ""},
	{"empty label", "[[begin:]] x [[end:]]", "x"},
	{"span inside a value", "acme/app#1 [[begin:title]] Onboarding [[end:title]]", "acme/app#1 Onboarding"},
	{"two spans", "move [[begin:title]] x [[end:title]] Status: [[begin:before]] a [[end:before]] -> [[begin:after]] b [[end:after]]", "move x Status: a -> b"},
	{"broken pairs stay broken", "[[begin:title]] x [ [end:title] ] y [[end:title]]", "x [ [end:title] ] y"},
	{"no close", "[[begin:title]] no close", "[[begin:title]] no close"},
	{"label mismatch", "[[begin:title]] x [[end:body]]", "[[begin:title]] x [[end:body]]"},
	{"no label end", "[[begin:title x", "[[begin:title x"},
	{"only open tag", "[[begin:title]]", "[[begin:title]]"},
	{"mismatch then a span", "[[begin:title]] x [[end:body]] [[begin:body]] y [[end:body]]", "[[begin:title]] x [[end:body]] y"},
}

func TestUndelimit(t *testing.T) {
	for _, tc := range undelimitCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := undelimit(tc.in); got != tc.want {
				t.Fatalf("undelimit(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

var truncateCellCases = []struct {
	cell  string
	width int
	want  string
}{
	{"plain text", 0, "plain text"},
	{"plain text", 20, "plain text"},
	{"plain text", 6, "pla..."},
	{"título longo", 8, "títul..."},
	{"plain text", 4, "p..."},
}

func TestTruncateCell(t *testing.T) {
	for _, tc := range truncateCellCases {
		if got := truncateCell(tc.cell, tc.width); got != tc.want {
			t.Errorf("truncateCell(%q, %d) = %q, want %q", tc.cell, tc.width, got, tc.want)
		}
	}
}
