package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var writeCapabilityCases = []struct {
	name string
	args []string
	edit func(*Deps, *writeFake)
	code domain.ExitCode
}{
	{"no estimate", []string{"estimate", "1", "1"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Estimate = nil }, 1},
	{"estimate text", []string{"estimate", "1", "1"}, func(_ *Deps, f *writeFake) { f.project.Fields[3].DataType = domain.DataTypeText }, 1},
	{"estimate missing", []string{"estimate", "1", "1"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Estimate.Field = "missing" }, 2},
	{"no sprint", []string{"sprint", "set", "1", "current"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Sprint = nil }, 1},
	{"sprint text", []string{"sprint", "set", "1", "current"}, func(_ *Deps, f *writeFake) { f.project.Fields[6].DataType = domain.DataTypeText }, 1},
	{"sprint missing", []string{"sprint", "set", "1", "current"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Sprint.Field = "missing" }, 2},
	{"sprint no active", []string{"sprint", "set", "1", "current"}, func(_ *Deps, f *writeFake) { f.project.Fields[6].Iterations = nil }, 2},
	{"no dates", []string{"dates", "1", "--start", "2026-10-01"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Dates = nil }, 1},
	{"date source", []string{"dates", "1", "--start", "2026-10-01"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Dates.Source = "unknown" }, 1},
	{"date text", []string{"dates", "1", "--start", "2026-10-01"}, func(_ *Deps, f *writeFake) { f.project.Fields[4].DataType = domain.DataTypeText }, 1},
	{"date missing", []string{"dates", "1", "--start", "2026-10-01"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Dates.Start = "missing" }, 2},
	{"date unmapped", []string{"dates", "1", "--start", "2026-10-01"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Dates.Start = "" }, 1},
	{"unsupported field", []string{"set", "1", "Epic=test"}, func(_ *Deps, f *writeFake) { f.project.Fields[2].DataType = domain.DataTypeTitle }, 1},
	{"existing parent", []string{"link", "1", "--parent", "2"}, func(_ *Deps, f *writeFake) {
		it := f.items["I_kwDOTest0001"]
		it.Issue.Parent = &domain.ParentRef{NodeID: "I_kwDOTest0003"}
		f.items["I_kwDOTest0001"] = it
	}, 3},
	{"no new repo", []string{"new", "task", "--title", "T"}, func(d *Deps, _ *writeFake) { d.Config.Config.Repository = "invalid" }, 1},
	{"unknown type", []string{"new", "task", "--title", "T"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Task.IssueType = "Taks" }, 2},
	{"no triage", []string{"new", "entry", "--title", "T"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Triage = nil }, 1},
	{"no lane", []string{"new", "task", "--title", "T", "--lane", "Platform"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Lane = nil }, 1},
	{"unknown lane", []string{"new", "task", "--title", "T", "--lane", "Platform"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Lane.Field = "missing" }, 2},
	{"no epic", []string{"new", "task", "--title", "T", "--epic", "E"}, func(d *Deps, _ *writeFake) { d.Config.Config.Capabilities.Epic = nil }, 1},
}

// A board whose owner has no issue types (a user account) still creates
// work: new makes a plain issue when no type is mapped.
func TestWriteNewWithoutIssueTypes(t *testing.T) {
	for _, tc := range []struct {
		kind string
		edit func(*Deps)
	}{
		{"task", func(d *Deps) { d.Config.Config.Capabilities.Task = nil }},
		{"task", func(d *Deps) { d.Config.Config.Capabilities.Task.IssueType = "" }},
		{"epic", func(d *Deps) { d.Config.Config.Capabilities.Epic = nil }},
		{"epic", func(d *Deps) { d.Config.Config.Capabilities.Epic.IssueType = "" }},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			tc.edit(deps)
			if err := writeTestExecute(context.Background(), []string{"new", tc.kind, "--title", "T"}, deps); err != nil {
				t.Fatal(err)
			}
			if !domain.Contains(fake.calls, "create") || fake.created.IssueTypeID != "" || fake.created.Title != "T" {
				t.Fatalf("created = %+v, calls = %v", fake.created, fake.calls)
			}
		})
	}
}

func TestWriteCapabilities(t *testing.T) {
	for _, tc := range writeCapabilityCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			tc.edit(deps, fake)
			err := writeTestExecute(context.Background(), tc.args, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("code=%d want=%d: %v", domain.CodeOf(err), tc.code, err)
			}
			writeAssertNoMutation(t, fake)
		})
	}
}

var writeDependencyCases = []struct {
	call string
	args []string
}{
	{"labels", []string{"label", "1", "+bug"}},
	{"milestones", []string{"milestone", "set", "1", "Release"}},
	{"users", []string{"assign", "1", "alice"}},
	{"userID", []string{"assign", "1", "carol"}},
	{"types", []string{"new", "task", "--title", "T"}},
	{"create", []string{"new", "task", "--title", "T"}},
	{"addProject", []string{"new", "task", "--title", "T"}},
	{"updateIssue", []string{"milestone", "set", "1", "Release"}},
	{"assign", []string{"assign", "1", "alice"}},
	{"unassign", []string{"unassign", "1", "bob"}},
	{"addLabels", []string{"label", "1", "+bug"}},
	{"removeLabels", []string{"label", "1", "-old"}},
	{"comment", []string{"comment", "1", "hello"}},
	{"link", []string{"link", "1", "--parent", "2"}},
}

func TestWriteDependencyFailures(t *testing.T) {
	for _, tc := range writeDependencyCases {
		t.Run(tc.call, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			fake.failure = tc.call
			err := writeTestExecute(context.Background(), tc.args, deps)
			if domain.CodeOf(err) != domain.ExitAPI {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestWriteOrganizationDates(t *testing.T) {
	for _, failure := range []string{"", "issueFieldID", "setIssueField", "updateIssueField"} {
		t.Run("dates "+failure, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			deps.Config.Config.Capabilities.Dates.Source = domain.DateSourceIssueFields
			fake.failure = failure
			err := writeTestExecute(context.Background(), []string{"dates", "1", "--start", "2026-10-08", "--target", "2026-10-09"}, deps)
			if (err != nil) != (failure != "") {
				t.Fatalf("%v", err)
			}
			if !strings.Contains(deps.Err.(*bytes.Buffer).String(), "visible in every project") {
				t.Fatal("missing scope warning")
			}
		})
	}
}
