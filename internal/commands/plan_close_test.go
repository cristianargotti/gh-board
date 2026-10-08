package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var planCloseCases = []struct {
	name  string
	args  []string
	state domain.IssueState
	code  domain.ExitCode
	saved bool
}{
	{"plan", []string{"close", "#7"}, domain.IssueOpen, domain.ExitPlanRequired, true},
	{"closed", []string{"close", "#7"}, domain.IssueClosed, domain.ExitOK, false},
	{"dry", []string{"close", "#7", "--dry-run", "--json"}, domain.IssueOpen, domain.ExitPlanRequired, false},
	{"bad ref", []string{"close", "not-a-ref"}, domain.IssueOpen, domain.ExitUsage, false},
	{"missing", []string{"close", "#9"}, domain.IssueOpen, domain.ExitNotFound, false},
	{"drift", []string{"close", "#7", "--expect", "Flow=Ready"}, domain.IssueOpen, domain.ExitDrift, false},
	{"bad format", []string{"close", "#7", "--format", "bad"}, domain.IssueOpen, domain.ExitUsage, false},
}

func TestPlanClose(t *testing.T) {
	for index, tc := range planCloseCases {
		t.Run(tc.name, func(t *testing.T) {
			planTestCloseCase(t, index)
		})
	}
}

func planTestCloseCase(t *testing.T, index int) {
	t.Helper()
	tc := planCloseCases[index]
	deps, reader, writer, out := planTestDeps(t)
	reader.items[0].Issue.State = tc.state
	err := planTestExecute(context.Background(), tc.args, deps)
	if domain.CodeOf(err) != tc.code {
		t.Fatalf("code = %v, error %v", domain.CodeOf(err), err)
	}
	if len(writer.calls) != 0 {
		t.Fatalf("unexpected writes: %v", writer.calls)
	}
	plans, err := plan.List(deps.Dirs.State)
	if err != nil || (len(plans) > 0) != tc.saved {
		t.Fatalf("plans = %v, %v", plans, err)
	}
	if tc.saved {
		planTestVerifyClose(t, plans[0])
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("raw terminal escape")
	}
}

func planTestVerifyClose(t *testing.T, p domain.Plan) {
	t.Helper()
	if err := p.Verify(); err != nil {
		t.Fatal(err)
	}
	if p.ExpiresAt.Sub(p.CreatedAt) != domain.PlanExpiry || p.Actor != "tester" || p.KitVersion != "test" {
		t.Fatalf("metadata: %+v", p)
	}
	if len(p.Steps) != 1 || p.Steps[0].Operation != domain.OpCloseIssue || p.Steps[0].Before != "OPEN" {
		t.Fatalf("steps: %+v", p.Steps)
	}
	// The file holds clean text: the issue reference, never the delimited
	// title, which the target carries and the renderer delimits.
	if !strings.Contains(p.Description, "team/work#7") || strings.Contains(p.Description, "[[") {
		t.Fatalf("description %q", p.Description)
	}
}

func TestPlanInspect(t *testing.T) {
	for _, verb := range []string{"list", "show"} {
		t.Run(verb, func(t *testing.T) {
			deps, _, _, out := planTestDeps(t)
			if err := planTestExecute(context.Background(), []string{"close", "#7"}, deps); domain.CodeOf(err) != domain.ExitPlanRequired {
				t.Fatal(err)
			}
			p := planTestSaved(t, deps)
			out.Reset()
			args := []string{"plan", verb, "--json"}
			if verb == "show" {
				args = append(args, p.ID)
			}
			if err := planTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), p.ID) || strings.Contains(out.String(), "\\u001b") {
				t.Fatalf("output = %q", out.String())
			}
		})
	}
}

func TestPlanSharedParents(t *testing.T) {
	for _, name := range []string{"digest", "milestone", "plan"} {
		t.Run(name, func(t *testing.T) {
			deps, _, _, _ := planTestDeps(t)
			root := NewRoot(deps)
			parent := commandParent(root, name, GroupRead)
			if commandParent(root, name, GroupPlan) != parent {
				t.Fatal("parent replaced")
			}
			count := 0
			for _, cmd := range root.Commands() {
				if cmd.Name() == name {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("parents = %d", count)
			}
		})
	}
}
