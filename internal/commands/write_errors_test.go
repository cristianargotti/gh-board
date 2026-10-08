package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

var writeInvalidCases = []struct {
	name string
	args []string
	code domain.ExitCode
	hint string
}{
	{"missing ref", []string{"move", "Active"}, 1, ""},
	{"bad ref", []string{"move", "%%%", "Active"}, 1, ""},
	{"missing target", []string{"move", "99", "Active"}, 2, ""},
	{"unknown option", []string{"move", "1", "Actve"}, 2, "did you mean"},
	{"unknown field", []string{"set", "1", "Siz=2"}, 2, "did you mean"},
	{"unknown login", []string{"assign", "1", "alce"}, 2, "did you mean"},
	{"unknown label", []string{"label", "1", "+bgu"}, 2, "did you mean"},
	{"unknown milestone", []string{"milestone", "set", "1", "Releas"}, 2, "did you mean"},
	{"bad pair", []string{"set", "1", "Size"}, 1, ""},
	{"empty field", []string{"set", "1", "=2"}, 1, ""},
	{"clear field", []string{"set", "1", "Epic="}, 1, ""},
	{"duplicate field", []string{"set", "1", "Size=1", "Size=2"}, 1, ""},
	{"nan estimate", []string{"estimate", "1", "NaN"}, 1, ""},
	{"negative estimate", []string{"estimate", "1", "-1"}, 1, ""},
	{"estimate limit", []string{"estimate", "1", "4"}, 3, ""},
	{"number invalid", []string{"set", "1", "Size=abc"}, 1, ""},
	{"date invalid", []string{"set", "1", "Start=today"}, 1, ""},
	{"date args", []string{"dates", "1"}, 1, ""},
	{"date parse", []string{"dates", "1", "--target", "today"}, 1, ""},
	{"date order", []string{"dates", "1", "--start", "2026-10-09", "--target", "2026-10-08"}, 1, ""},
	{"iteration missing", []string{"sprint", "set", "1", "Cycly 1"}, 2, "did you mean"},
	{"comment empty", []string{"comment", "1", " "}, 1, ""},
	{"parent missing", []string{"link", "1"}, 1, ""},
	{"parent self", []string{"link", "1", "--parent", "1"}, 1, ""},
	{"label syntax", []string{"label", "1", "bug"}, 1, ""},
	{"label duplicate", []string{"label", "1", "+bug", "-bug"}, 1, ""},
	{"label missing", []string{"label", "1"}, 1, ""},
	{"label flag", []string{"label", "1", "+bug", "--no-such"}, 1, ""},
	{"new title", []string{"new", "task"}, 1, ""},
	{"new kind", []string{"new", "thing", "--title", "Task"}, 1, ""},
	{"new expect", []string{"new", "task", "--title", "Task", "--expect", "Flow=Ready"}, 1, ""},
	{"expect mismatch", []string{"comment", "1", "Hello", "--expect", "Flow=Done"}, 4, ""},
	{"expect invalid", []string{"comment", "1", "Hello", "--expect", "Flow"}, 1, ""},
	{"expect unknown", []string{"comment", "1", "Hello", "--expect", "Flw=Ready"}, 2, "did you mean"},
	{"expect conflict", []string{"comment", "1", "Hello", "--expect", "Flow=Ready", "--expect", "Flow=Active"}, 1, ""},
	{"bad format", []string{"comment", "1", "Hello", "--format", "unknown"}, 1, ""},
	{"bad project", []string{"comment", "1", "Hello", "--project", "invalid"}, 1, ""},
}

func TestWriteInvalidInput(t *testing.T) {
	for _, tc := range writeInvalidCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			err := writeTestExecute(context.Background(), tc.args, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("code=%d want=%d: %v", domain.CodeOf(err), tc.code, err)
			}
			if tc.hint != "" && !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("missing hint: %v", err)
			}
			writeAssertNoMutation(t, fake)
		})
	}
}

func writeAssertNoMutation(t *testing.T, fake *writeFake) {
	t.Helper()
	for _, name := range []string{"create", "setField", "assign", "unassign", "addLabels", "removeLabels", "comment", "link", "reopen", "restore", "updateIssue", "setIssueField", "updateIssueField"} {
		if domain.Contains(fake.calls, name) {
			t.Fatalf("unexpected mutation %s: %v", name, fake.calls)
		}
	}
}

var writeRefusalCases = []struct {
	name string
	edit func(*Deps, *writeFake)
	code domain.ExitCode
}{
	{"repository permission", func(_ *Deps, f *writeFake) { f.perm = domain.PermissionRead }, 3},
	{"project permission", func(_ *Deps, f *writeFake) { f.project.ViewerRole = domain.RoleReader }, 3},
	{"transition", func(d *Deps, _ *writeFake) { d.Config.Config.Policy.Transitions["Ready"] = []string{"Done"} }, 3},
	{"WIP", func(d *Deps, f *writeFake) {
		d.Config.Config.Policy.WIP["Active"] = 1
		writeSetStatus(f, "I_kwDOTest0002", "Active")
	}, 3},
	{"status drift preflight", func(_ *Deps, f *writeFake) {
		f.changeAt = 2
		f.change = func(f *writeFake) { writeSetStatus(f, "I_kwDOTest0001", "Done") }
	}, 4},
	{"status drift immediately", func(_ *Deps, f *writeFake) {
		f.changeAt = 3
		f.change = func(f *writeFake) { writeSetStatus(f, "I_kwDOTest0001", "Done") }
	}, 4},
	{"apply refused propagation", func(_ *Deps, f *writeFake) { f.failure = "read"; f.failureErr = domain.ErrApplyRefused }, 6},
	{"API", func(_ *Deps, f *writeFake) { f.failure = "discover" }, 7},
	{"no project", func(d *Deps, _ *writeFake) { d.Config.Config.Project = domain.ProjectRef{} }, 1},
	{"no repository", func(d *Deps, _ *writeFake) { d.Config.Config.Repository = "" }, 1},
	{"no status", func(_ *Deps, f *writeFake) { f.project.Fields = nil }, 1},
}

func writeSetStatus(fake *writeFake, id, status string) {
	it := fake.items[id]
	values := map[string]domain.FieldValue{}
	for field, value := range it.Values {
		values[field] = value
	}
	values["Flow"] = domain.FieldValue{Field: "Flow", Value: status}
	it.Values = values
	fake.items[id] = it
}

func TestWriteRefusals(t *testing.T) {
	for _, tc := range writeRefusalCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			tc.edit(deps, fake)
			err := writeTestExecute(context.Background(), []string{"move", "1", "Active"}, deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("code=%d want=%d: %v", domain.CodeOf(err), tc.code, err)
			}
			writeAssertNoMutation(t, fake)
			entries, auditErr := audit.Query(deps.Dirs.State, time.Time{})
			if auditErr != nil || len(entries) != 1 || entries[0].Error == "" {
				t.Fatalf("audit=%v err=%v", entries, auditErr)
			}
		})
	}
}

func TestWritePortFailures(t *testing.T) {
	for _, call := range []string{"viewer", "permission", "list", "read", "setField"} {
		t.Run(call, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			fake.failure = call
			err := writeTestExecute(context.Background(), []string{"move", "1", "Active"}, deps)
			if domain.CodeOf(err) != domain.ExitAPI {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestWriteRefusedAuditTargets(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	deps.Config.Config.Policy.Transitions["Ready"] = []string{"Done"}
	err := writeTestExecute(context.Background(), []string{"move", "1", "Active"}, deps)
	if domain.CodeOf(err) != domain.ExitPolicy {
		t.Fatalf("%v", err)
	}
	entries, err := audit.Query(deps.Dirs.State, time.Time{})
	if err != nil || len(entries) != 1 || !domain.Contains(entries[0].Targets, fake.items["I_kwDOTest0001"].Issue.NodeID) {
		t.Fatalf("missing refused target: %+v, %v", entries, err)
	}
}
