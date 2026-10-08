package guard_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

type normalizeCase struct {
	name string
	in   []string
	want []string
}

var gh = []string{"gh"}

var normalizeCases = []normalizeCase{
	{"assignments", []string{"FOO=1", "BAR=x y", "gh", "project", "delete", "1"}, []string{"gh", "project", "delete", "1"}},
	{"array assignment", []string{"A[0]=1", "gh"}, gh},
	{"append assignment", []string{"PATH+=:x", "gh"}, gh},
	{"timeout", []string{"timeout", "30", "gh"}, gh},
	{"timeout options", []string{"timeout", "-k", "5", "--signal=KILL", "-s", "TERM", "--foreground", "30", "gh", "x"}, []string{"gh", "x"}},
	{"timeout alone", []string{"timeout"}, []string{"timeout"}},
	{"timeout without command", []string{"timeout", "30"}, nil},
	{"time", []string{"time", "-p", "gh"}, gh},
	{"time format", []string{"time", "-f", "%e", "-o", "t.txt", "gh"}, gh},
	{"nice", []string{"nice", "-n", "10", "gh"}, gh},
	{"nice attached", []string{"nice", "-n10", "gh"}, gh},
	{"nohup", []string{"nohup", "gh"}, gh},
	{"nohup alone", []string{"nohup"}, nil},
	{"stdbuf", []string{"stdbuf", "-oL", "-e", "0", "--input=0", "gh"}, gh},
	{"command", []string{"command", "-p", "gh"}, gh},
	{"command query", []string{"command", "-v", "gh"}, []string{"command", "-v", "gh"}},
	{"command query capital", []string{"command", "-V", "gh"}, []string{"command", "-V", "gh"}},
	{"builtin", []string{"builtin", "gh"}, gh},
	{"noglob", []string{"noglob", "gh"}, gh},
	{"nocorrect", []string{"nocorrect", "gh"}, gh},
	{"exec", []string{"exec", "-a", "name", "-c", "gh"}, gh},
	{"sudo", []string{"sudo", "-u", "bob", "-E", "--", "gh"}, gh},
	{"doas", []string{"doas", "-u", "bob", "gh"}, gh},
	{"setsid", []string{"setsid", "-f", "gh"}, gh},
	{"ionice", []string{"ionice", "-c", "3", "gh"}, gh},
	{"xargs bare", []string{"xargs", "gh"}, gh},
	{"xargs options", []string{"xargs", "-n1", "gh"}, []string{"xargs", "-n1", "gh"}},
	{"keywords", []string{"if", "!", "gh"}, gh},
	{"brace", []string{"{", "gh"}, gh},
	{"done alone", []string{"done"}, nil},
	{"redirect attached", []string{"2>/dev/null", "gh"}, gh},
	{"redirect separate", []string{">", "out.txt", "gh"}, gh},
	{"redirect dangling", []string{">"}, nil},
	{"ampersand redirect", []string{"&>/dev/null", "gh"}, gh},
	{"input redirect", []string{"<", "in.txt", "gh"}, gh},
	{"stacked", []string{"FOO=1", "timeout", "5", "nice", "sudo", "gh", "project", "delete", "1"}, []string{"gh", "project", "delete", "1"}},
	{"not a wrapper", []string{"gh", "project", "list"}, []string{"gh", "project", "list"}},
	{"wrapper in argument position", []string{"echo", "timeout", "gh"}, []string{"echo", "timeout", "gh"}},
	{"empty", nil, nil},
}

func TestNormalize(t *testing.T) {
	for _, c := range normalizeCases {
		t.Run(c.name, func(t *testing.T) {
			got := guard.Normalize(guard.Segment(c.in)).String()
			if want := strings.Join(c.want, " "); got != want {
				t.Fatalf("Normalize(%q) = %q, want %q", c.in, got, want)
			}
		})
	}
}
