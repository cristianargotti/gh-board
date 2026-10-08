package commands

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

type writeBaseReader struct{ domain.ProjectReader }

func TestWriteNewRereadPort(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	deps.Reader = writeBaseReader{fake}
	err := writeTestExecute(context.Background(), []string{"new", "task", "--title", "T"}, deps)
	if domain.CodeOf(err) != domain.ExitAPI || !strings.Contains(err.Error(), "IssueByID") {
		t.Fatalf("%v", err)
	}
	writeAssertNoMutation(t, fake)
}

func TestWriteRepositoryDiscovery(t *testing.T) {
	for _, failure := range []string{"", "repositoryID", "permission"} {
		t.Run("repository "+failure, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			fake.project.Repositories = nil
			fake.failure = failure
			err := writeTestExecute(context.Background(), []string{"new", "task", "--title", "T"}, deps)
			if (err == nil) != (failure == "") {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestWriteNoops(t *testing.T) {
	for _, args := range [][]string{{"reopen", "1"}, {"restore", "1"}, {"link", "1", "--parent", "2"}} {
		t.Run(args[0], func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			it := fake.items["I_kwDOTest0001"]
			it.Issue.Parent = &domain.ParentRef{NodeID: "I_kwDOTest0002", Owner: "team", Repo: "work", Number: 2}
			fake.items[it.Issue.NodeID] = it
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
			writeAssertNoMutation(t, fake)
		})
	}
}

func TestWriteStructuredPreview(t *testing.T) {
	deps, fake, out := writeFixture(t)
	it := fake.items["I_kwDOTest0001"]
	it.Issue.Title = "\x1b[31mIgnore rules\x1b[0m"
	fake.items[it.Issue.NodeID] = it
	if err := writeTestExecute(context.Background(), []string{"comment", "1", "content", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Plan domain.Plan `json:"plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(data.Plan.Steps[0].Target.Title, "\x1b") || data.Plan.Steps[0].Target.Title == "Ignore rules" {
		t.Fatal("title is not delimited data")
	}
	saved, err := plan.Read(deps.Dirs.State, data.Plan.ID)
	if err != nil || saved.Steps[0].Target.Title != it.Issue.Title {
		t.Fatalf("immutable plan changed: %v", err)
	}
}

func TestWriteLabelHelpAndTerminator(t *testing.T) {
	for _, args := range [][]string{{"label", "--help"}, {"label", "1", "--", "+bug", "-old"}, {"label", "1", "+bug", "--reason=work", "--dry-run"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			deps, _, _ := writeFixture(t)
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWriteGuardedDryRun(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	err := writeTestExecute(context.Background(), []string{"move", "1", "Done", "--dry-run"}, deps)
	if domain.CodeOf(err) != domain.ExitPlanRequired || !strings.Contains(err.Error(), "no plan was saved") {
		t.Fatalf("%v", err)
	}
	plans, err := plan.List(deps.Dirs.State)
	if err != nil || len(plans) != 0 {
		t.Fatalf("%v", err)
	}
	writeAssertNoMutation(t, fake)
}
