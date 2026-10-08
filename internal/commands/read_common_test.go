package commands

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// readGenericDeps builds dependencies without board.yml: the project comes
// from the override alone.
func readGenericDeps(t *testing.T, reader domain.ProjectReader) (*Deps, *strings.Builder) {
	t.Helper()
	deps, _ := readTestDeps(t, reader)
	out := &strings.Builder{}
	deps.Out = out
	deps.Config = &config.Loaded{
		Config: &domain.Config{Version: config.SupportedVersion, Project: domain.ProjectRef{Owner: "acme", Number: 7}},
		Source: config.SourceGeneric,
	}
	return deps, out
}

func TestReadLoadFromConfigFlag(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	deps.Config = nil
	path := filepath.Join(t.TempDir(), "board.yml")
	if err := os.WriteFile(path, []byte(readFixtureYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := readRun(t, deps, "--config", path, "status"); err != nil {
		t.Fatal(err)
	}
	if deps.Config == nil || deps.Config.Source != config.SourceFlag {
		t.Fatalf("config = %+v", deps.Config)
	}
	if !strings.Contains(out.String(), "Board acme/7") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestReadLoadGenericMode(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	deps.Config = nil
	t.Setenv(config.EnvConfig, "")
	if err := readRun(t, deps, "--project", "acme/7", "status"); err != nil {
		t.Fatal(err)
	}
	if deps.Config == nil || !deps.Config.Generic() {
		t.Fatalf("config = %+v", deps.Config)
	}
	if !strings.Contains(out.String(), "generic mode") {
		t.Fatalf("output = %s", out.String())
	}
}

var readLoadErrorCases = []struct {
	name   string
	args   []string
	code   domain.ExitCode
	noConf bool
}{
	{"bad project flag", []string{"--project", "nope", "status"}, domain.ExitUsage, false},
	{"missing config file", []string{"--config", filepath.Join("testdata", "nope.yml"), "status"}, domain.ExitUsage, true},
	{"bad format", []string{"--format", "yaml", "status"}, domain.ExitUsage, false},
	{"unknown project", []string{"--project", "acme/9", "status"}, domain.ExitNotFound, false},
	{"bad format version", []string{"--format", "xml", "version"}, domain.ExitUsage, false},
	{"bad format log", []string{"--format", "xml", "log"}, domain.ExitUsage, false},
	{"bad format doctor", []string{"--format", "xml", "doctor"}, domain.ExitUsage, false},
	{"bad format schema", []string{"--format", "xml", "schema"}, domain.ExitUsage, false},
}

func TestReadLoadErrors(t *testing.T) {
	for _, tc := range readLoadErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, _ := readTestDeps(t, readNewFakeReader())
			if tc.noConf {
				deps.Config = nil
			}
			if got := readRunCode(t, deps, tc.args...); got != tc.code {
				t.Fatalf("exit = %v, want %v", got, tc.code)
			}
		})
	}
}

func TestReadNoProjectAndNoReader(t *testing.T) {
	deps, _ := readTestDeps(t, readNewFakeReader())
	deps.Config = &config.Loaded{Config: &domain.Config{Version: config.SupportedVersion}, Source: config.SourceGeneric}
	for _, args := range [][]string{{"status"}, {"schema"}} {
		if got := readRunCode(t, deps, args...); got != domain.ExitUsage {
			t.Fatalf("%v without project: exit = %v", args, got)
		}
	}
	deps, _ = readTestDeps(t, nil)
	for _, args := range [][]string{{"status"}, {"schema"}} {
		if got := readRunCode(t, deps, args...); got != domain.ExitUsage {
			t.Fatalf("%v without reader: exit = %v", args, got)
		}
	}
}

func TestReadJSONFlag(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	if err := readRun(t, deps, "--json", "version"); err != nil {
		t.Fatal(err)
	}
	var payload readVersionPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || payload.Version != "1.2.3" {
		t.Fatalf("payload = %+v, err = %v", payload, err)
	}
	deps.Version = ""
	out.Reset()
	if err := readRun(t, deps, "version"); err != nil || !strings.Contains(out.String(), "gh board dev") {
		t.Fatalf("output = %s, err = %v", out.String(), err)
	}
}

var readAPICases = []struct {
	name string
	err  error
	code domain.ExitCode
}{
	{"nil", nil, domain.ExitOK},
	{"plain", errors.New("boom"), domain.ExitAPI},
	{"not found", domain.ErrNotFound, domain.ExitNotFound},
	{"ambiguous", domain.ErrAmbiguous, domain.ExitNotFound},
	{"usage", domain.ErrUsage, domain.ExitUsage},
	{"coded", domain.Errorf(domain.ExitPolicy, "no"), domain.ExitPolicy},
}

func TestReadAPI(t *testing.T) {
	for _, tc := range readAPICases {
		if got := domain.CodeOf(readAPI(tc.err)); got != tc.code {
			t.Errorf("%s: exit = %v, want %v", tc.name, got, tc.code)
		}
	}
}

func TestReadPagination(t *testing.T) {
	fake := readNewFakeReader()
	fake.pageSize = 4
	deps, out := readTestDeps(t, fake)
	if err := readRun(t, deps, "--json", "status"); err != nil {
		t.Fatal(err)
	}
	var payload readStatusPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Items.Open != 7 || payload.Items.Closed != 1 || payload.Items.Archived != 1 {
		t.Fatalf("items = %+v", payload.Items)
	}
	fake.stuck = true
	for _, args := range [][]string{{"status"}, {"list", "--all"}} {
		if got := readRunCode(t, deps, args...); got != domain.ExitAPI {
			t.Fatalf("%v with a stuck cursor: exit = %v", args, got)
		}
	}
}

func TestReadParent(t *testing.T) {
	root := &cobra.Command{Use: "board"}
	first := commandParent(root, "sprint", GroupRead)
	second := commandParent(root, "sprint", GroupWrite)
	if first != second || first.GroupID != GroupRead || len(root.Commands()) != 1 {
		t.Fatalf("parent reused = %v, group = %q, commands = %d", first == second, first.GroupID, len(root.Commands()))
	}
}

func TestReadHelpers(t *testing.T) {
	deps := &Deps{}
	if d := time.Since(deps.Now()); d < 0 || d > time.Minute {
		t.Fatalf("Now without a clock drifted by %v", d)
	}
	if deps.Now().Location() != time.UTC || deps.Now().Nanosecond() != 0 {
		t.Fatal("Now must be UTC at second precision")
	}
	if readCount(1, "item", "items") != "1 item" || readCount(2, "item", "items") != "2 items" {
		t.Fatal("readCount")
	}
	if readYesNo(true) != "yes" || readYesNo(false) != "no" {
		t.Fatal("readYesNo")
	}
	users := []domain.User{{Login: "ana"}, {Login: "bruno"}}
	if readJoin(readLoginList(users)) != "ana, bruno" {
		t.Fatal("logins")
	}
	labels := []domain.Label{{Name: "bug"}, {Name: "docs"}}
	if readJoin(readLabelList(labels)) != "bug, docs" {
		t.Fatal("labels")
	}
	if readProgress(0, 0) != "0/0" || readProgress(1, 3) != "1/3 (33%)" {
		t.Fatal("progress")
	}
	if readStamp(readFixtureNow) != "2026-10-08T12:00:00Z" {
		t.Fatal("dates")
	}
	if readSkippedNote(0) != "" || readSkippedNote(1) != "1 board item omitted: draft issues, pull requests or redacted items" {
		t.Fatal("skipped note")
	}
}
