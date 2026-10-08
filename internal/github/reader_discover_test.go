package github_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

var (
	acmeBoard = domain.ProjectRef{Owner: "acme", Number: 7}
	sandbox   = domain.ProjectRef{Owner: "membro1", Number: 2}
)

var discoverCases = []struct {
	dir       string
	ref       domain.ProjectRef
	nodeID    string
	title     string
	fields    int
	items     int
	role      domain.Role
	views     int
	workflows int
	repos     int
}{
	{"acme", acmeBoard, "PVT_6gozs2PV0g9mjnL3", "Time Produto", 32, 169, domain.RoleWriter, 9, 13, 6},
	{"sandbox", sandbox, "PVT_wNsk5eGXHNnHmD7K", "gh-board sandbox (cópia de Time Produto)", 32, 0, domain.RoleAdmin, 9, 8, 1},
	{"template", domain.ProjectRef{Owner: "acme", Number: 24}, "PVT_VDlOjkUW8bidBVTR", "[TEMPLATE] Acme", 20, 0, domain.RoleWriter, 5, 6, 0},
	{"devops", domain.ProjectRef{Owner: "acme", Number: 26}, "PVT__mGFEy6M_Z_9BlrI", "Time Infra", 20, 12, domain.RoleWriter, 5, 6, 0},
	{"sec", domain.ProjectRef{Owner: "acme", Number: 35}, "PVT_Q5eNoBY20XdsmLCE", "Time Segurança", 13, 1, domain.RoleWriter, 1, 6, 0},
}

func TestDiscoverProjectBoards(t *testing.T) {
	for _, tc := range discoverCases {
		t.Run(tc.dir, func(t *testing.T) {
			_, a := newReplay(t, tc.dir)
			p, err := a.DiscoverProject(context.Background(), tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			if p.NodeID != tc.nodeID || p.Title != tc.title || p.Ref != tc.ref || p.ViewerRole != tc.role {
				t.Fatalf("project = %s %q %s %s", p.NodeID, p.Title, p.Ref, p.ViewerRole)
			}
			if len(p.Fields) != tc.fields || p.ItemCount != tc.items || len(p.Views) != tc.views ||
				len(p.Workflows) != tc.workflows || len(p.Repositories) != tc.repos {
				t.Fatalf("counts: fields %d items %d views %d workflows %d repos %d",
					len(p.Fields), p.ItemCount, len(p.Views), len(p.Workflows), len(p.Repositories))
			}
			if !p.DiscoveredAt.Equal(testNow) {
				t.Fatalf("DiscoveredAt = %s", p.DiscoveredAt)
			}
		})
	}
}

func TestDiscoverProjectSchemaDetails(t *testing.T) {
	_, a := newReplay(t, "acme")
	p, err := a.DiscoverProject(context.Background(), acmeBoard)
	if err != nil {
		t.Fatal(err)
	}
	assertStatusField(t, p)
	sprint, ok := p.FieldByName("Sprint")
	if !ok || sprint.DataType != domain.DataTypeIteration || len(sprint.Iterations) != 9 {
		t.Fatalf("Sprint = %+v", sprint)
	}
	if sprint.Iterations[0].Title != "Sprint 1" || !sprint.Iterations[0].Completed || sprint.Iterations[4].Completed {
		t.Fatalf("iterations are not chronological: %+v", sprint.Iterations)
	}
	current, ok := sprint.IterationAt(testNow)
	if !ok || current.Title != "Sprint 5" || current.Duration != 14 || !current.Start.Equal(time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("current iteration = %+v", current)
	}
	assertBoardStructure(t, p)
}

func assertStatusField(t *testing.T, p domain.Project) {
	t.Helper()
	status, ok := p.FieldByName("Status")
	if !ok || status.DataType != domain.DataTypeSingleSelect || len(status.Options) != 7 || status.Options[0].Name != "BACKLOG" {
		t.Fatalf("Status = %+v", status)
	}
	if _, ok := status.OptionByName("DONE"); !ok {
		t.Fatal("DONE option missing")
	}
}

func assertBoardStructure(t *testing.T, p domain.Project) {
	t.Helper()
	if v := p.Views[0]; v.Number != 2 || v.Name != "Tarefas" || v.Layout != "BOARD_LAYOUT" || !strings.Contains(v.Filter, "-type:Feature") {
		t.Fatalf("view = %+v", v)
	}
	if w := p.Workflows[0]; w.Name != "Auto-add conector" || !w.Enabled || w.NodeID == "" {
		t.Fatalf("workflow = %+v", w)
	}
	if r := p.Repositories[0]; r.FullName() != "acme/precos-api" || r.NodeID == "" {
		t.Fatalf("repository = %+v", r)
	}
	if !strings.Contains(p.README, "README sintético") {
		t.Fatalf("README = %q", p.README)
	}
}

var discoverErrorCases = []struct {
	name string
	ref  domain.ProjectRef
}{
	{"project number missing", domain.ProjectRef{Owner: "acme", Number: 999999}},
	{"owner missing", domain.ProjectRef{Owner: "login-that-does-not-exist-xyz", Number: 1}},
}

func TestDiscoverProjectNotFound(t *testing.T) {
	for _, tc := range discoverErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, a := newReplay(t, "errors")
			_, err := a.DiscoverProject(context.Background(), tc.ref)
			if !errors.Is(err, domain.ErrNotFound) || domain.CodeOf(err) != domain.ExitNotFound {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}
		})
	}
}

func TestDiscoverProjectCache(t *testing.T) {
	r, a := newReplay(t, "acme")
	ctx := context.Background()
	first, err := a.DiscoverProject(ctx, acmeBoard)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.DiscoverProject(ctx, acmeBoard)
	if err != nil || second.NodeID != first.NodeID || r.count() != 1 {
		t.Fatalf("second discovery must come from the cache: err %v, requests %d", err, r.count())
	}
	if _, err := a.DiscoverProject(ctx, domain.ProjectRef{Owner: "Acme", Number: 7}); err != nil || r.count() != 1 {
		t.Fatalf("the cache ignores the login case: err %v, requests %d", err, r.count())
	}
	if _, err := a.RefreshProject(ctx, acmeBoard); err != nil || r.count() != 2 {
		t.Fatalf("refresh must bypass the cache: err %v, requests %d", err, r.count())
	}
	assertOwnerOnly(t, cacheFile(t, a))
	later := github.New(github.Options{CacheDir: a.Options().CacheDir, Transport: a.Options().Transport, Clock: fixedClock{now: testNow.Add(2 * time.Hour)}})
	if _, err := later.DiscoverProject(ctx, acmeBoard); err != nil || r.count() != 3 {
		t.Fatalf("an expired entry must be refetched: err %v, requests %d", err, r.count())
	}
}

func assertOwnerOnly(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache file mode = %o, want 600", info.Mode().Perm())
	}
}

// cacheFile returns the single schema file the adapter wrote.
func cacheFile(t *testing.T, a *github.Adapter) string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(a.Options().CacheDir, "schema", "*.json"))
	if len(files) != 1 {
		t.Fatalf("cache files = %v", files)
	}
	return files[0]
}

func rewriteCache(t *testing.T, a *github.Adapter, from, to string) {
	t.Helper()
	data, err := os.ReadFile(cacheFile(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), from) {
		t.Fatalf("cache does not contain %q", from)
	}
	edited := []byte(strings.Replace(string(data), from, to, 1))
	if err := os.WriteFile(cacheFile(t, a), edited, 0o600); err != nil { //nolint:gosec // the cache file sits under the test temporary directory
		t.Fatal(err)
	}
}

var cacheMissCases = []struct {
	name  string
	spoil func(t *testing.T, a *github.Adapter)
}{
	{"corrupted file", func(t *testing.T, a *github.Adapter) {
		t.Helper()
		if err := os.WriteFile(cacheFile(t, a), []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
	}},
	{"other host", func(t *testing.T, a *github.Adapter) {
		rewriteCache(t, a, `"host":"github.com"`, `"host":"ghe.example.com"`)
	}},
	{"future discovery", func(t *testing.T, a *github.Adapter) {
		rewriteCache(t, a, `"discovered_at":"2026-10-08T12:00:00Z"`, `"discovered_at":"2030-01-01T00:00:00Z"`)
	}},
}

func TestDiscoverProjectCacheMisses(t *testing.T) {
	for _, tc := range cacheMissCases {
		t.Run(tc.name, func(t *testing.T) {
			r, a := newReplay(t, "acme")
			ctx := context.Background()
			if _, err := a.DiscoverProject(ctx, acmeBoard); err != nil {
				t.Fatal(err)
			}
			tc.spoil(t, a)
			if _, err := a.DiscoverProject(ctx, acmeBoard); err != nil || r.count() != 2 {
				t.Fatalf("spoiled cache must be refetched: err %v, requests %d", err, r.count())
			}
		})
	}
}

func TestDiscoverProjectWithoutCacheDir(t *testing.T) {
	r, a := newReplay(t, "acme")
	noCache := github.New(github.Options{Transport: a.Options().Transport, Clock: fixedClock{now: testNow}})
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		if _, err := noCache.DiscoverProject(ctx, acmeBoard); err != nil || r.count() != i {
			t.Fatalf("without a cache directory every call fetches: err %v, requests %d", err, r.count())
		}
	}
	blocker := filepath.Join(t.TempDir(), "file-not-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	unwritable := github.New(github.Options{CacheDir: filepath.Join(blocker, "cache"), Transport: a.Options().Transport, Clock: fixedClock{now: testNow}})
	if _, err := unwritable.DiscoverProject(ctx, acmeBoard); err != nil {
		t.Fatalf("an unwritable cache must not break discovery: %v", err)
	}
}
