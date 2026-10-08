package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

type planTestBrokenWriter struct{}

func (planTestBrokenWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

var planTargetErrorCases = []struct {
	name, reference string
	configure       func(*Deps, *planTestReader)
	code            domain.ExitCode
}{
	{"repository", "#7", func(d *Deps, _ *planTestReader) { d.Config.Config.Repository = "" }, domain.ExitUsage},
	{"bad repository", "#7", func(d *Deps, _ *planTestReader) { d.Config.Config.Repository = "invalid" }, domain.ExitUsage},
	{"item", "#7", func(_ *Deps, r *planTestReader) { r.fail["item"] = errors.New("offline") }, domain.ExitAPI},
	{"empty node", "#7", func(_ *Deps, r *planTestReader) { r.items[0].Issue.NodeID = "" }, domain.ExitNotFound},
	{"permission api", "#7", func(_ *Deps, r *planTestReader) { r.fail["permission"] = errors.New("offline") }, domain.ExitAPI},
	{"permission denied", "#7", func(_ *Deps, r *planTestReader) { r.permission = domain.PermissionRead }, domain.ExitPolicy},
}

func TestPlanTargetErrors(t *testing.T) {
	for _, tc := range planTargetErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			tc.configure(deps, reader)
			pc := &planContext{deps: deps, cfg: deps.Config.Config, project: reader.project}
			_, err := pc.planItem(context.Background(), tc.reference)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPlanOutputErrors(t *testing.T) {
	for _, format := range []string{"table", "json", "bogus", ""} {
		t.Run("format-"+format, func(t *testing.T) {
			deps, _, _, _ := planTestDeps(t)
			deps.Flags.Format = format
			deps.Out = planTestBrokenWriter{}
			if err := planRender(deps, render.NewDocument("Plan")); err == nil {
				t.Fatal("expected output error")
			}
		})
	}
}

func TestPlanPersistenceErrors(t *testing.T) {
	for _, kind := range []string{"plan", "audit", "output"} {
		t.Run(kind, func(t *testing.T) {
			planTestPersistenceCase(t, kind)
		})
	}
}

func planTestPersistenceCase(t *testing.T, kind string) {
	t.Helper()
	deps, reader, _, _ := planTestDeps(t)
	pc := &planContext{deps: deps, cfg: deps.Config.Config, project: reader.project, viewer: reader.viewer}
	if kind == "output" {
		deps.Out = planTestBrokenWriter{}
	} else {
		if err := os.MkdirAll(deps.Dirs.State, 0o700); err != nil {
			t.Fatal(err)
		}
		name := "plans"
		if kind == "audit" {
			name = "audit.jsonl"
		}
		path := filepath.Join(deps.Dirs.State, name)
		if kind == "audit" {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	step := domain.Step{Operation: domain.OpCloseIssue, Target: planTarget(reader.items[0]), Before: "OPEN", After: "CLOSED"}
	err := pc.planSave("close", "Fechar a issue.", []domain.Step{step})
	if err == nil || domain.CodeOf(err) == domain.ExitPlanRequired {
		t.Fatalf("result = %v", err)
	}
}

func TestPlanInspectErrors(t *testing.T) {
	for _, command := range []string{"list", "show"} {
		t.Run(command, func(t *testing.T) {
			deps, _, _, _ := planTestDeps(t)
			args := []string{"plan", command}
			if command == "show" {
				args = append(args, "missing")
			} else {
				if err := os.MkdirAll(deps.Dirs.State, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(deps.Dirs.State, "plans"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := planTestExecute(context.Background(), args, deps); err == nil {
				t.Fatal("missing error")
			}
		})
	}
}

func TestPlanMilestoneInputErrors(t *testing.T) {
	for _, repository := range []string{"invalid", "team/work"} {
		t.Run(repository, func(t *testing.T) {
			input, err := planMilestoneInput(repository, "Title", "Description", "")
			if (err == nil) != (repository == "team/work") {
				t.Fatal(err)
			}
			if err == nil && input.DueOn != nil {
				t.Fatal("invented deadline")
			}
		})
	}
}

func TestPlanPermissionOnItemSets(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "denied", true: "allowed"}[allowed], func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			if !allowed {
				reader.permission = domain.PermissionRead
			}
			pc := &planContext{deps: deps, cfg: deps.Config.Config}
			items := append(reader.items, reader.items[0])
			err := pc.planCheckItems(context.Background(), items)
			if (err == nil) != allowed {
				t.Fatal(err)
			}
		})
	}
}
