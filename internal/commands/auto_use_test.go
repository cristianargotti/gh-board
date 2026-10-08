package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestAutoUseRecordShowClear(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	path := config.DefaultPath(deps.Dirs.Config)
	if err := readRun(t, deps, "use", "acme/7"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Project:  acme/7") || !strings.Contains(out.String(), "Time Produto") || !strings.Contains(out.String(), path) {
		t.Fatalf("record output = %s", out.String())
	}
	def, err := config.ReadDefault(deps.Dirs.Config)
	if err != nil || def.Project != (domain.ProjectRef{Owner: "acme", Number: 7}) {
		t.Fatalf("recorded default = %+v, %v", def, err)
	}
	out.Reset()
	if err := readRun(t, deps, "--json", "use"); err != nil {
		t.Fatal(err)
	}
	var payload autoUsePayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || !payload.Recorded || payload.Project != "acme/7" || payload.Path != path {
		t.Fatalf("show payload = %+v, %v", payload, err)
	}
	out.Reset()
	if err := readRun(t, deps, "use", "--clear"); err != nil || !strings.Contains(out.String(), "default project cleared") {
		t.Fatalf("clear: %v %s", err, out.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("default file must be gone: %v", err)
	}
	out.Reset()
	if err := readRun(t, deps, "use", "--clear"); err != nil || !strings.Contains(out.String(), "no default project recorded") {
		t.Fatalf("second clear: %v %s", err, out.String())
	}
	out.Reset()
	if err := readRun(t, deps, "use"); err != nil || !strings.Contains(out.String(), "no default project: run gh board use owner/number") {
		t.Fatalf("show without default: %v %s", err, out.String())
	}
}

var autoUseErrorCases = []struct {
	name  string
	args  []string
	setup func(t *testing.T, deps *Deps)
	code  domain.ExitCode
}{
	{"malformed project", []string{"use", "nope"}, func(*testing.T, *Deps) {}, domain.ExitUsage},
	{"unknown project is not recorded", []string{"use", "acme/9"}, func(*testing.T, *Deps) {}, domain.ExitNotFound},
	{"clear with a project", []string{"use", "acme/7", "--clear"}, func(*testing.T, *Deps) {}, domain.ExitUsage},
	{"no config directory", []string{"use", "acme/7"}, func(_ *testing.T, deps *Deps) { deps.Dirs.Config = "" }, domain.ExitUsage},
	{"no reader", []string{"use", "acme/7"}, func(_ *testing.T, deps *Deps) { deps.Reader = nil }, domain.ExitUsage},
	{"damaged default on show", []string{"use"}, func(t *testing.T, deps *Deps) {
		readWriteFile(t, config.DefaultPath(deps.Dirs.Config), "project: 1\n")
	}, domain.ExitUsage},
	{"directory in the way of clear", []string{"use", "--clear"}, func(t *testing.T, deps *Deps) {
		if err := os.MkdirAll(filepath.Join(config.DefaultPath(deps.Dirs.Config), "x"), 0o700); err != nil {
			t.Fatal(err)
		}
	}, domain.ExitUsage},
}

func TestAutoUseErrors(t *testing.T) {
	for _, tc := range autoUseErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, _ := readTestDeps(t, readNewFakeReader())
			tc.setup(t, deps)
			if got := readRunCode(t, deps, tc.args...); got != tc.code {
				t.Fatalf("exit = %v, want %v", got, tc.code)
			}
			if tc.name == "unknown project is not recorded" {
				if _, err := os.Stat(config.DefaultPath(deps.Dirs.Config)); !os.IsNotExist(err) {
					t.Fatalf("a project that does not resolve must not be recorded: %v", err)
				}
			}
		})
	}
}

// TestReadLoadDefaultProject runs a read with no flag and no board.yml:
// the recorded default selects the project, and two per-user files
// without a default name themselves in the error.
func TestReadLoadDefaultProject(t *testing.T) {
	readPinEnvironment(t)
	deps, out := readTestDeps(t, readNewFakeReader())
	deps.Config = nil
	readWriteFile(t, config.DefaultPath(deps.Dirs.Config), string(config.EncodeDefault(domain.ProjectRef{Owner: "acme", Number: 7})))
	if err := readRun(t, deps, "status"); err != nil {
		t.Fatal(err)
	}
	if deps.Config.Source != config.SourceDefault || !deps.Config.Generic() || !strings.Contains(out.String(), "Board acme/7") {
		t.Fatalf("source = %q output = %s", deps.Config.Source, out.String())
	}
	deps, _ = readTestDeps(t, readNewFakeReader())
	deps.Config = nil
	readWriteFile(t, filepath.Join(deps.Dirs.Config, "acme-7.yml"), readFixtureYAML)
	readWriteFile(t, filepath.Join(deps.Dirs.Config, "other-1.yml"), "version: 1\n")
	err := readRun(t, deps, "status")
	if domain.CodeOf(err) != domain.ExitUsage || !strings.Contains(err.Error(), "2 per-user files (acme-7.yml, other-1.yml)") || !strings.Contains(err.Error(), "gh board use owner/number") {
		t.Fatalf("two candidates: %v", err)
	}
	for _, args := range [][]string{{"schema"}, {"close", "acme/app#1"}, {"watch", "--once"}, {"tidy"}} {
		if err := readRun(t, deps, args...); domain.CodeOf(err) != domain.ExitUsage || !strings.Contains(err.Error(), "gh board use owner/number") {
			t.Fatalf("%v without a project: %v", args, err)
		}
	}
}
