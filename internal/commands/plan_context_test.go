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
)

var planContextCases = []struct {
	name   string
	change func(*Deps, *planTestReader)
	code   domain.ExitCode
}{
	{"viewer", func(_ *Deps, r *planTestReader) { r.fail["viewer"] = errors.New("offline") }, domain.ExitAPI},
	{"identity", func(_ *Deps, r *planTestReader) { r.viewer.Host = "" }, domain.ExitAPI},
	{"shadow", func(_ *Deps, r *planTestReader) { r.viewer.TokenSource = "GH_TOKEN" }, domain.ExitOK},
	{"clock", func(d *Deps, _ *planTestReader) { d.Clock = nil }, domain.ExitUsage},
	{"reader", func(d *Deps, _ *planTestReader) { d.Reader = nil }, domain.ExitUsage},
	{"state", func(d *Deps, _ *planTestReader) { d.Dirs.State = "" }, domain.ExitUsage},
	{"project", func(d *Deps, _ *planTestReader) { d.Config.Config.Project = domain.ProjectRef{} }, domain.ExitUsage},
	{"bad override", func(d *Deps, _ *planTestReader) { d.Flags.Project = "bad" }, domain.ExitUsage},
	{"override", func(d *Deps, _ *planTestReader) { d.Flags.Project = "team/9" }, domain.ExitOK},
	{"discover", func(_ *Deps, r *planTestReader) { r.fail["discover"] = errors.New("offline") }, domain.ExitAPI},
	{"role", func(_ *Deps, r *planTestReader) { r.project.ViewerRole = domain.RoleReader }, domain.ExitPolicy},
}

func TestPlanContext(t *testing.T) {
	for _, tc := range planContextCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			tc.change(deps, reader)
			_, err := planStart(context.Background(), deps)
			if domain.CodeOf(err) != tc.code {
				t.Fatalf("error = %v", err)
			}
			if tc.name == "shadow" && !strings.Contains(deps.Err.(*bytes.Buffer).String(), "shadows") {
				t.Fatal("shadow warning missing")
			}
		})
	}
}

func TestPlanLoadConfiguration(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "valid", false: "invalid"}[valid], func(t *testing.T) {
			deps, _, _, _ := planTestDeps(t)
			deps.Config = nil
			deps.Flags.Config = filepath.Join(t.TempDir(), "board.yml")
			data := "version: 1\nproject: {owner: team, number: 9}\n"
			if !valid {
				data = "version: 0\n"
			}
			if err := os.WriteFile(deps.Flags.Config, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := planLoad(deps)
			if (err == nil) != valid {
				t.Fatalf("config %+v: %v", cfg, err)
			}
			if valid && cfg.Project.Number != 9 {
				t.Fatal(cfg)
			}
		})
	}
}

func TestPlanGenericConfig(t *testing.T) {
	for _, cfg := range []*domain.Config{nil, {Version: 1}} {
		t.Run("generic", func(t *testing.T) {
			deps, _, _, _ := planTestDeps(t)
			deps.Config = &config.Loaded{Config: cfg, Source: config.SourceGeneric}
			actual, err := planLoad(deps)
			if err != nil || actual.Version != 1 {
				t.Fatalf("config %+v, %v", actual, err)
			}
		})
	}
}

var planPaginationCases = []struct {
	name  string
	pages map[string]domain.ItemPage
	code  domain.ExitCode
	count int
}{
	{"all", map[string]domain.ItemPage{"": {Items: []domain.Item{{}}, HasNext: true, NextCursor: "next"}, "next": {Items: []domain.Item{{}}}}, domain.ExitOK, 2},
	{"empty cursor", map[string]domain.ItemPage{"": {HasNext: true}}, domain.ExitAPI, 0},
	{"loop", map[string]domain.ItemPage{"": {HasNext: true, NextCursor: "next"}, "next": {HasNext: true, NextCursor: "next"}}, domain.ExitAPI, 0},
}

func TestPlanPagination(t *testing.T) {
	for _, tc := range planPaginationCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			reader.pages = tc.pages
			pc := &planContext{deps: deps, project: reader.project}
			items, err := pc.planItems(context.Background())
			if domain.CodeOf(err) != tc.code || len(items) != tc.count {
				t.Fatalf("items %d: %v", len(items), err)
			}
		})
	}
}

var planExpectationCases = []struct {
	field string
	code  domain.ExitCode
}{
	{"Flow=Doing", domain.ExitOK},
	{"state=OPEN", domain.ExitOK},
	{"milestone=", domain.ExitOK},
	{"Inicio=2026-10-08", domain.ExitOK},
	{"Flow=Wrong", domain.ExitDrift},
	{"Missing=value", domain.ExitNotFound},
	{"=empty", domain.ExitUsage},
	{"bare", domain.ExitUsage},
}

func TestPlanExpectations(t *testing.T) {
	for _, tc := range planExpectationCases {
		t.Run(tc.field, func(t *testing.T) {
			_, reader, _, _ := planTestDeps(t)
			item := reader.items[0]
			item.IssueFields = []domain.IssueFieldValue{{Name: "Inicio", Value: "2026-10-08"}}
			if err := planExpect(item, []string{tc.field}); domain.CodeOf(err) != tc.code {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanJSONFailure(t *testing.T) {
	for _, value := range []any{make(chan int), func() {}} {
		t.Run("unsupported", func(t *testing.T) {
			if _, err := planJSON(value); err == nil {
				t.Fatal("expected encode failure")
			}
		})
	}
}
