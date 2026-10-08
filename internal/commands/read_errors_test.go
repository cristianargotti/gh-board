package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var readFailureCases = []struct {
	name string
	args []string
	fail string
	code domain.ExitCode
}{
	{"context discover", []string{"context"}, "DiscoverProject", domain.ExitAPI},
	{"context viewer", []string{"context"}, "Viewer", domain.ExitAPI},
	{"context items", []string{"context"}, "ListItems", domain.ExitAPI},
	{"status items", []string{"status"}, "ListItems", domain.ExitAPI},
	{"me viewer", []string{"me"}, "Viewer", domain.ExitAPI},
	{"me items", []string{"me"}, "ListItems", domain.ExitAPI},
	{"item get", []string{"item", "acme/app#3"}, "GetItem", domain.ExitAPI},
	{"list items", []string{"list"}, "ListItems", domain.ExitAPI},
	{"epics items", []string{"epics"}, "ListItems", domain.ExitAPI},
	{"roadmap items", []string{"roadmap"}, "ListItems", domain.ExitAPI},
	{"roadmap milestones", []string{"roadmap"}, "RepositoryMilestones", domain.ExitAPI},
	{"sprint items", []string{"sprint", "current"}, "ListItems", domain.ExitAPI},
	{"attention items", []string{"attention"}, "ListItems", domain.ExitAPI},
	{"search items", []string{"search", "x"}, "ListItems", domain.ExitAPI},
	{"schema discover", []string{"schema"}, "DiscoverProject", domain.ExitAPI},
	{"digest items", []string{"digest", "print"}, "ListItems", domain.ExitAPI},
	{"digest milestones", []string{"digest", "print"}, "RepositoryMilestones", domain.ExitAPI},
	{"standup items", []string{"standup"}, "ListItems", domain.ExitAPI},
}

func TestReadFailures(t *testing.T) {
	for _, tc := range readFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			fake := readNewFakeReader()
			fake.fail[tc.fail] = errors.New("boom")
			deps, _ := readTestDeps(t, fake)
			if got := readRunCode(t, deps, tc.args...); got != tc.code {
				t.Fatalf("exit = %v, want %v", got, tc.code)
			}
		})
	}
}

var readArgumentCases = []struct {
	name string
	args []string
	code domain.ExitCode
	want string
}{
	{"item malformed", []string{"item", "nope!"}, domain.ExitUsage, ""},
	{"item not found", []string{"item", "acme/app#99"}, domain.ExitNotFound, ""},
	{"item node id", []string{"item", "I_kwDOAbc004"}, domain.ExitOK, "Blocked value:  Bloqueada"},
	{"item url", []string{"item", "https://github.com/acme/app/issues/6"}, domain.ExitOK, "Closed:      2026-10-07T00:00:00Z"},
	{"item milestone", []string{"item", "#2"}, domain.ExitOK, "Milestone:   v1.0 (OPEN, due 2026-12-15)"},
	{"item archived", []string{"item", "#7"}, domain.ExitOK, "archived item"},
	{"list limit zero", []string{"list", "--limit", "0"}, domain.ExitUsage, ""},
	{"list sprint title", []string{"list", "--sprint", "Sprint 4"}, domain.ExitOK, "no items match"},
	{"list sprint next", []string{"list", "--sprint", "next"}, domain.ExitOK, "0 items shown"},
	{"list values", []string{"list", "--all", "--blocked", "--lane", "dados", "--epic", "alertas", "--type", "task"}, domain.ExitOK, "acme/app#4"},
	{"list triage", []string{"list", "--triage", "--label", "urgente"}, domain.ExitOK, "acme/app#8"},
	{"roadmap months zero", []string{"roadmap", "--months", "0"}, domain.ExitUsage, ""},
	{"roadmap months big", []string{"roadmap", "--months", "25"}, domain.ExitUsage, ""},
	{"search empty", []string{"search", " "}, domain.ExitUsage, ""},
	{"search label", []string{"search", "URGENTE"}, domain.ExitOK, "label"},
	{"search ref", []string{"search", "app#9"}, domain.ExitOK, "ref"},
	{"search none", []string{"search", "zzz"}, domain.ExitOK, "Matches:  0"},
	{"digest bad week", []string{"digest", "print", "--week", "2026-41"}, domain.ExitUsage, ""},
	{"log bad since", []string{"log", "--since", "yesterday"}, domain.ExitUsage, ""},
	{"standup for", []string{"standup", "--for", "carol"}, domain.ExitOK, "nothing moved yesterday"},
	{"sprint title", []string{"sprint", "current", "--json"}, domain.ExitOK, `"which":"current"`},
}

func TestReadArguments(t *testing.T) {
	for _, tc := range readArgumentCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, out := readTestDeps(t, readNewFakeReader())
			if got := readRunCode(t, deps, tc.args...); got != tc.code {
				t.Fatalf("exit = %v, want %v (%s)", got, tc.code, out.String())
			}
			if tc.want != "" && !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output lacks %q:\n%s", tc.want, out.String())
			}
		})
	}
}

var readGenericCases = []struct {
	name string
	args []string
	code domain.ExitCode
	want string
}{
	{"list lane", []string{"list", "--lane", "x"}, domain.ExitUsage, ""},
	{"list epic", []string{"list", "--epic", "x"}, domain.ExitUsage, ""},
	{"list sprint", []string{"list", "--sprint", "current"}, domain.ExitUsage, ""},
	{"list overdue", []string{"list", "--overdue"}, domain.ExitUsage, ""},
	{"list triage", []string{"list", "--triage"}, domain.ExitUsage, ""},
	{"list blocked", []string{"list", "--blocked"}, domain.ExitOK, "acme/app#4"},
	{"item short", []string{"item", "3"}, domain.ExitUsage, ""},
	{"item", []string{"item", "acme/app#3"}, domain.ExitOK, "dates unavailable"},
	{"epics", []string{"epics"}, domain.ExitOK, "epic unavailable"},
	{"sprint", []string{"sprint", "current"}, domain.ExitOK, "sprint unavailable"},
	{"roadmap", []string{"roadmap"}, domain.ExitOK, "no iteration markers"},
	{"attention", []string{"attention"}, domain.ExitOK, "generic mode"},
	{"standup", []string{"standup"}, domain.ExitOK, "status classes are needed"},
	{"context", []string{"context"}, domain.ExitOK, "generic mode"},
	{"status", []string{"status"}, domain.ExitOK, "generic mode"},
	{"digest", []string{"digest", "print"}, domain.ExitOK, "Nenhuma métrica declarada"},
	{"me", []string{"me"}, domain.ExitOK, "acme/app#1"},
}

func TestReadGenericMode(t *testing.T) {
	for _, tc := range readGenericCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, out := readGenericDeps(t, readNewFakeReader())
			if got := readRunCode(t, deps, tc.args...); got != tc.code {
				t.Fatalf("exit = %v, want %v (%s)", got, tc.code, out.String())
			}
			if tc.want != "" && !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output lacks %q:\n%s", tc.want, out.String())
			}
		})
	}
}

// readStatusWithoutField covers a board whose status field is missing.
func TestReadStatusWithoutStatusField(t *testing.T) {
	fake := readNewFakeReader()
	fake.project.Fields = fake.project.Fields[:1]
	deps, out := readTestDeps(t, fake)
	if err := readRun(t, deps, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no single select field named Status") || !strings.Contains(out.String(), "IN PROGRESS") {
		t.Fatalf("output = %s", out.String())
	}
}

// readSelectionCases pair each read with the selection it sends: the
// listings, the alerts and the snapshot read summaries; item, search,
// roadmap and digest need the body or the milestone and read everything.
var readSelectionCases = []struct {
	args      []string
	selection domain.ItemSelection
}{
	{[]string{"context"}, domain.SelectionSummary},
	{[]string{"status"}, domain.SelectionSummary},
	{[]string{"attention"}, domain.SelectionSummary},
	{[]string{"list"}, domain.SelectionSummary},
	{[]string{"list", "--all", "--overdue"}, domain.SelectionSummary},
	{[]string{"me"}, domain.SelectionSummary},
	{[]string{"epics"}, domain.SelectionSummary},
	{[]string{"sprint", "current"}, domain.SelectionSummary},
	{[]string{"standup"}, domain.SelectionSummary},
	{[]string{"search", "login"}, domain.SelectionFull},
	{[]string{"roadmap"}, domain.SelectionFull},
	{[]string{"digest", "print"}, domain.SelectionFull},
}

func TestReadSelections(t *testing.T) {
	for _, tc := range readSelectionCases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			fake := readNewFakeReader()
			deps, _ := readTestDeps(t, fake)
			if err := readRun(t, deps, tc.args...); err != nil {
				t.Fatal(err)
			}
			if len(fake.selections) == 0 {
				t.Fatal("no items were read")
			}
			for _, got := range fake.selections {
				if got != tc.selection {
					t.Fatalf("selection = %q, want %q", got, tc.selection)
				}
			}
		})
	}
}
