package commands

import (
	"context"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestWriteCreationResumesFromJournal(t *testing.T) {
	for _, failure := range []string{"addProject", "setField"} {
		t.Run(failure, func(t *testing.T) {
			writeAssertCreationResume(t, failure)
		})
	}
}

func writeAssertCreationResume(t *testing.T, failure string) {
	t.Helper()
	deps, fake, _ := writeFixture(t)
	fake.failure = failure
	err := writeTestExecute(context.Background(), []string{"new", "task", "--title", "T", "--estimate", "1"}, deps)
	if domain.CodeOf(err) != domain.ExitAPI {
		t.Fatalf("%v", err)
	}
	plans, err := plan.List(deps.Dirs.State)
	if err != nil || len(plans) != 1 {
		t.Fatalf("%v", err)
	}
	fake.failure, fake.calls = "", nil
	result, err := plan.Apply(context.Background(), plan.Ports{Reader: fake, Writer: fake, Clock: deps.Clock}, plans[0],
		plan.ApplyOptions{Interactive: true, Viewer: fake.viewer, StateDir: deps.Dirs.State, Out: deps.Out})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Done) == 0 || domain.Contains(fake.calls, "create") {
		t.Fatalf("repeated creation: %+v, %v", result, fake.calls)
	}
	if fake.items["I_kwDOTest0004"].Text("Size") != "1" {
		t.Fatal("resume did not complete the estimate")
	}
}
