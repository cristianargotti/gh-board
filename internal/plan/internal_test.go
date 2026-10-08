package plan

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var setCases = []struct {
	name   string
	values []string
	joined string
	split  []string
}{
	{name: "empty", values: nil, joined: "", split: nil},
	{name: "sorted and trimmed", values: []string{" b", "a ", ""}, joined: "a,b", split: []string{"a", "b"}},
	{name: "single", values: []string{"x"}, joined: "x", split: []string{"x"}},
}

func TestJoinSplitSet(t *testing.T) {
	for _, tc := range setCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JoinSet(tc.values); got != tc.joined {
				t.Fatalf("JoinSet = %q, want %q", got, tc.joined)
			}
			if got := SplitSet(tc.joined); strings.Join(got, "|") != strings.Join(tc.split, "|") {
				t.Fatalf("SplitSet = %q, want %q", got, tc.split)
			}
		})
	}
	if got := difference([]string{"A", "b", "c"}, []string{"a", "C"}); len(got) != 1 || got[0] != "b" {
		t.Fatalf("difference = %q", got)
	}
	if owner, name := splitRepo("acme/repo"); owner != "acme" || name != "repo" {
		t.Fatalf("splitRepo = %q, %q", owner, name)
	}
}

var labelCases = []struct {
	name   string
	target domain.Target
	want   string
}{
	{name: "issue with title", target: domain.Target{Repository: "acme/repo", Number: 12, Title: "Fix\tlogin\x1b[0m"}, want: `acme/repo#12 "Fix login [0m"`},
	{name: "repository only", target: domain.Target{Repository: "acme/repo"}, want: "acme/repo"},
	{name: "node id", target: domain.Target{NodeID: "PVT_1"}, want: "PVT_1"},
	{name: "long title is cut", target: domain.Target{NodeID: "I_1", Title: strings.Repeat("x", 90)}, want: "I_1 \"" + strings.Repeat("x", 80) + "...\""},
}

func TestLabel(t *testing.T) {
	for _, tc := range labelCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Label(tc.target); got != tc.want {
				t.Fatalf("Label = %q, want %q", got, tc.want)
			}
		})
	}
}

var refCases = []struct {
	id    string
	index int
	ok    bool
}{
	{id: "step:0", index: 0, ok: true},
	{id: "step:12", index: 12, ok: true},
	{id: "step:-1", ok: false},
	{id: "step:x", ok: false},
	{id: "I_1", ok: false},
	{id: "", ok: false},
}

func TestRefs(t *testing.T) {
	for _, tc := range refCases {
		index, ok := parseRef(tc.id)
		if ok != tc.ok || (ok && index != tc.index) {
			t.Errorf("parseRef(%q) = %d, %v", tc.id, index, ok)
		}
	}
	if Ref(3) != "step:3" || !isRef(Ref(3)) {
		t.Fatal("Ref is wrong")
	}
	payload, err := EncodePayload(domain.CreateLabelInput{Name: "lane"})
	if err != nil || payload != `{"repository_id":"","name":"lane"}` {
		t.Fatalf("EncodePayload = %q, %v", payload, err)
	}
	if _, err := EncodePayload(make(chan int)); err == nil {
		t.Fatal("an unencodable payload must fail")
	}
	var in domain.CreateLabelInput
	if err := decodePayload(domain.Step{After: "{"}, &in); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("decodePayload: %v", err)
	}
}

var describeCases = []struct {
	name string
	step domain.Step
	want string
}{
	{name: "precondition", step: domain.Step{Index: 0, Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: "I_1"}, Field: "Status", Before: "Todo", After: "Done"}, want: `1. set_field_value I_1 Status: "Todo" -> "Done"`},
	{name: "creation", step: domain.Step{Index: 1, Operation: domain.OpCreateLabel, Target: domain.Target{Repository: "acme/repo"}, After: `{"Name":"x"}`}, want: `2. create_label acme/repo: {"Name":"x"}`},
	{name: "bare", step: domain.Step{Index: 2, Operation: domain.OpCopyProject}, want: "3. copy_project "},
}

func TestDescribe(t *testing.T) {
	for _, tc := range describeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := describe(tc.step); got != tc.want {
				t.Fatalf("describe = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMarkers(t *testing.T) {
	if Marker("p1", 2) != "<!-- gh-board plan=p1 step=2 -->" {
		t.Fatal("Marker is wrong")
	}
	monday := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	if WeekMarker(monday) != "<!-- gh-board digest=2026-W41 -->" {
		t.Fatalf("WeekMarker = %q", WeekMarker(monday))
	}
	if !SameISOWeek(monday, monday.AddDate(0, 0, 6)) || SameISOWeek(monday, monday.AddDate(0, 0, 7)) {
		t.Fatal("SameISOWeek is wrong")
	}
}

func TestIsTerminal(t *testing.T) {
	if IsTerminal(nil) {
		t.Fatal("nil is not a terminal")
	}
	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if IsTerminal(f) {
		t.Fatal("a regular file is not a terminal")
	}
}

func TestInsertSortedAndResult(t *testing.T) {
	list := insertSorted(nil, 2)
	list = insertSorted(list, 0)
	list = insertSorted(list, 2)
	if len(list) != 2 || list[0] != 0 || list[1] != 2 {
		t.Fatalf("insertSorted = %v", list)
	}
	res := ApplyResult{PlanID: "p1", DryRun: true}
	res.add(StatusDone, 1)
	res.add(StatusSkipped, 0)
	if res.String() != "plan p1 dry run: 1 done, 1 skipped" {
		t.Fatalf("String = %q", res.String())
	}
}
