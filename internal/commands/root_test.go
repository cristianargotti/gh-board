package commands_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/commands"
	"github.com/cristianargotti/gh-board/internal/domain"
)

func newDeps() (*commands.Deps, *bytes.Buffer, *bytes.Buffer) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	return &commands.Deps{Out: out, Err: errOut, Version: "test"}, out, errOut
}

func TestNewRoot(t *testing.T) {
	deps, _, _ := newDeps()
	root := commands.NewRoot(deps)
	if root.Name() != "board" {
		t.Fatalf("Name() = %q", root.Name())
	}
	for _, flag := range []string{"project", "config", "json", "format", "dry-run", "reason", "expect"} {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("missing global flag --%s", flag)
		}
	}
	groups := root.Groups()
	if len(groups) != 4 {
		t.Fatalf("expected the four tier groups, got %d", len(groups))
	}
	if groups[0].ID != commands.GroupRead || groups[3].ID != commands.GroupAuto {
		t.Fatalf("group order = %v", groups)
	}
}

func TestExecuteHelp(t *testing.T) {
	deps, out, _ := newDeps()
	err := commands.Execute(context.Background(), []string{"--help"}, deps)
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out.String(), "board") || !strings.Contains(out.String(), "--dry-run") {
		t.Fatalf("help output = %q", out.String())
	}
}

func TestExecuteBindsGlobalFlags(t *testing.T) {
	deps, _, _ := newDeps()
	args := []string{
		"--project", "acme/7", "--config", "b.yml", "--json", "--format", "md",
		"--dry-run", "--reason", "limpeza", "--expect", "Status=DONE", "--expect", "Sprint=5", "--help",
	}
	if err := commands.Execute(context.Background(), args, deps); err != nil {
		t.Fatalf("execute: %v", err)
	}
	f := deps.Flags
	if f.Project != "acme/7" || f.Config != "b.yml" || !f.JSON || f.Format != "md" || !f.DryRun || f.Reason != "limpeza" {
		t.Fatalf("flags = %+v", f)
	}
	if len(f.Expect) != 2 || f.Expect[1] != "Sprint=5" {
		t.Fatalf("expect = %v", f.Expect)
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	deps, _, _ := newDeps()
	err := commands.Execute(context.Background(), []string{"explode"}, deps)
	if err == nil {
		t.Fatal("unknown command must fail")
	}
	if domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("exit code = %v", domain.CodeOf(err))
	}
	if err := commands.Execute(context.Background(), []string{"--no-such-flag"}, deps); err == nil {
		t.Fatal("unknown flag must fail")
	}
}
