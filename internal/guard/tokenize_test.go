package guard_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/guard"
)

type tokenizeCase struct {
	name    string
	command string
	want    [][]string
}

var tokenizeCases = []tokenizeCase{
	{"simple", "gh project delete 46 --owner acme", [][]string{{"gh", "project", "delete", "46", "--owner", "acme"}}},
	{"and", "cd /tmp && gh project delete 46", [][]string{{"cd", "/tmp"}, {"gh", "project", "delete", "46"}}},
	{"separators", "a; b | c || d & e |& f\ng", [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}}},
	{"substitution", `echo "$(gh project delete 46)"`, [][]string{{"gh", "project", "delete", "46"}, {"echo"}}},
	{"backtick", "echo `gh project delete 46`", [][]string{{"gh", "project", "delete", "46"}, {"echo"}}},
	{"subshell", "(gh project delete 46)", [][]string{{"gh", "project", "delete", "46"}}},
	{"process substitution", "diff <(gh project delete 1) b", [][]string{{"diff", "<"}, {"gh", "project", "delete", "1"}, {"b"}}},
	{"quotes", `gh 'project' "delete" 46`, [][]string{{"gh", "project", "delete", "46"}}},
	{"escaped space", `gh\ project delete`, [][]string{{"gh project", "delete"}}},
	{"double quoted escape", `gh "pro\"ject" '$x'`, [][]string{{"gh", `pro"ject`, "$x"}}},
	{"line continuation", "gh project \\\ndelete 1", [][]string{{"gh", "project", "delete", "1"}}},
	{"empty quotes", "gh '' x", [][]string{{"gh", "x"}}},
	{"unterminated quote", "gh 'project delete", [][]string{{"gh", "project delete"}}},
	{"trailing backslash", "gh \\", [][]string{{"gh"}}},
	{"crlf", "gh project list\r\ngh project delete 1", [][]string{{"gh", "project", "list"}, {"gh", "project", "delete", "1"}}},
	{"dangling and", "a &&", [][]string{{"a"}}},
	{"empty", "", nil},
	{"blank", "   \t ", nil},
	{"here string", "cat <<< x\ngh project delete 1", [][]string{{"cat", "<<<", "x"}, {"gh", "project", "delete", "1"}}},
}

var heredocCases = []tokenizeCase{
	{
		"attached delimiter",
		"gh api graphql -f query=@- <<'EOF'\nmutation { deleteProjectV2(input: {projectId: PVT_x}) { clientMutationId } }\nEOF\necho done",
		[][]string{
			{"gh", "api", "graphql", "-f", "query=@-", "<<EOF", "mutation", "{", "deleteProjectV2(input:", "{projectId:", "PVT_x})", "{", "clientMutationId", "}", "}"},
			{"echo", "done"},
		},
	},
	{"dash and separate delimiter", "cat <<- EOF\n\tbody line\n\tEOF\necho x", [][]string{{"cat", "<<-", "EOF", "body", "line"}, {"echo", "x"}}},
	{"pipe keeps the body on the reader", "cat <<EOF | gh api graphql -f query=@-\nmutation deleteProjectV2\nEOF", [][]string{{"cat", "<<EOF", "mutation", "deleteProjectV2"}, {"gh", "api", "graphql", "-f", "query=@-"}}},
	{"inside substitution", "x=$(cat <<EOF\nbody\nEOF\n); gh project list", [][]string{{"cat", "<<EOF", "body"}, {"x="}, {"gh", "project", "list"}}},
	{"unterminated", "cat <<EOF\nbody", [][]string{{"cat", "<<EOF", "body"}}},
	{"two bodies", "diff <<A <<B\na\nA\nb\nB\n", [][]string{{"diff", "<<A", "<<B", "a", "b"}}},
	{"crlf body", "cat <<EOF\r\nbody\r\nEOF\r\necho x", [][]string{{"cat", "<<EOF", "body"}, {"echo", "x"}}},
	{"substitution before the newline", "cat <<EOF $(id)\nbody\nEOF", [][]string{{"id"}, {"cat", "<<EOF", "body"}}},
}

func TestTokenize(t *testing.T) {
	for _, c := range append(append([]tokenizeCase(nil), tokenizeCases...), heredocCases...) {
		t.Run(c.name, func(t *testing.T) {
			got := guard.Tokenize(c.command)
			if !equalSegments(got, c.want) {
				t.Fatalf("Tokenize(%q)\n got %q\nwant %q", c.command, got, c.want)
			}
		})
	}
}

func equalSegments(got []guard.Segment, want [][]string) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	segs := make([][]string, len(got))
	for i := range got {
		segs[i] = []string(got[i])
	}
	return reflect.DeepEqual(segs, want)
}

func TestTokenizeDeep(t *testing.T) {
	command := strings.Repeat("$(", 100) + "gh project delete 1" + strings.Repeat(")", 100)
	for _, seg := range guard.Tokenize(command) {
		if seg.String() == "gh project delete 1" {
			return
		}
	}
	t.Fatalf("the nested command was lost")
}

func TestSegmentString(t *testing.T) {
	if got := (guard.Segment{"gh", "project", "delete"}).String(); got != "gh project delete" {
		t.Fatalf("String() = %q", got)
	}
}
