package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/spf13/cobra"
)

type writeBrokenIO struct{}

func (writeBrokenIO) Read([]byte) (int, error)  { return 0, errors.New("broken input") }
func (writeBrokenIO) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestWriteBodySources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ source, input, want string }{{"literal", "", "literal"}, {path, "", "from file"}, {"@" + path, "", "from file"}, {"-", "from stdin", "from stdin"}}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			deps, _, _ := writeFixture(t)
			root := writeTestRoot(deps)
			root.SetIn(strings.NewReader(tc.input))
			root.SetArgs([]string{"comment", "1", tc.source})
			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatal(err)
			}
			plans, err := plan.List(deps.Dirs.State)
			if err != nil || len(plans) != 1 || plans[0].Steps[0].After != tc.want {
				t.Fatalf("%v: %+v", err, plans)
			}
		})
	}
}

func TestWriteBodyFailures(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		input        string
	}{{"missing", "@/missing/write-body", ""}, {"oversized", "-", strings.Repeat("x", writeBodyLimit+1)}} {
		t.Run(tc.name, func(t *testing.T) {
			root := &cobra.Command{}
			root.SetIn(strings.NewReader(tc.input))
			_, err := writeBody(root, tc.source)
			if domain.CodeOf(err) != domain.ExitUsage {
				t.Fatalf("%v", err)
			}
		})
	}
	if _, err := writeReadBody(writeBrokenIO{}); err == nil {
		t.Fatal("expected input error")
	}
}

func TestWriteLoadConfiguration(t *testing.T) {
	for _, project := range []string{"", "team/2"} {
		t.Run(project, func(t *testing.T) {
			deps, _, _ := writeFixture(t)
			path := filepath.Join(t.TempDir(), "board.yml")
			body := "version: 1\nproject:\n  owner: team\n  number: 1\nrepository: team/work\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			deps.Config = nil
			args := []string{"comment", "1", "Hello", "--config", path}
			if project != "" {
				args = append(args, "--project", project)
			}
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
			if deps.Config.Path != path {
				t.Fatalf("config path %s", deps.Config.Path)
			}
		})
	}
}

func TestWriteRootAndModes(t *testing.T) {
	for _, format := range []string{"table", "compact", "md"} {
		t.Run(format, func(t *testing.T) {
			deps, fake, out := writeFixture(t)
			fake.viewer.TokenSource = "GH_TOKEN"
			args := []string{"comment", "1", "Hello", "--format", format, "--reason", "Review", "--expect", "Flow=Ready"}
			if err := writeTestExecute(context.Background(), args, deps); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(deps.Err.(*bytes.Buffer).String(), "shadows") || out.Len() == 0 {
				t.Fatal("missing identity report or output")
			}
		})
	}
	root := &cobra.Command{Use: "board"}
	parent := commandParent(root, "sprint", GroupRead)
	if commandParent(root, "sprint", GroupRead) != parent || !parent.ContainsGroup(GroupWrite) {
		t.Fatal("parent not shared")
	}
}

func TestWriteLocalFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Deps)
	}{
		{"reader", func(d *Deps) { d.Reader = nil }},
		{"writer", func(d *Deps) { d.Writer = nil }},
		{"clock", func(d *Deps) { d.Clock = nil }},
		{"state", func(d *Deps) { d.Dirs.State = "" }},
		{"output", func(d *Deps) { d.Out = writeBrokenIO{} }},
		{"config", func(d *Deps) { d.Config = &config.Loaded{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _ := writeFixture(t)
			tc.edit(deps)
			if err := writeTestExecute(context.Background(), []string{"comment", "1", "hello"}, deps); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
