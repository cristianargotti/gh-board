package plan_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func TestWriteAndRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	p := newPlan(t, closeStep())
	p.Hash, p.ExpiresAt = "", time.Time{}
	p.Steps[0].Index = 9
	path, err := plan.Write(dir, p)
	if err != nil || path != plan.Path(dir, p.ID) {
		t.Fatalf("Write = %q, %v", path, err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Fatalf("permissions = %o", info.Mode().Perm())
		}
	}
	for _, ref := range []string{p.ID, path} {
		got, err := plan.Read(dir, ref)
		if err != nil {
			t.Fatalf("Read(%q): %v", ref, err)
		}
		if got.Hash == "" || got.Steps[0].Index != 0 || !got.ExpiresAt.Equal(now.Add(domain.PlanExpiry)) {
			t.Fatalf("read plan = %+v", got)
		}
	}
	if _, err := plan.Write(dir, p); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a plan is immutable, got %v", err)
	}
}

var writeFailures = []struct {
	name string
	edit func(p *domain.Plan)
}{
	{name: "invalid id", edit: func(p *domain.Plan) { p.ID = "../escape" }},
	{name: "empty id", edit: func(p *domain.Plan) { p.ID = "" }},
	{name: "no creation time", edit: func(p *domain.Plan) { p.CreatedAt = time.Time{} }},
}

func TestWriteFailures(t *testing.T) {
	for _, tc := range writeFailures {
		t.Run(tc.name, func(t *testing.T) {
			p := newPlan(t, closeStep())
			tc.edit(&p)
			if _, err := plan.Write(t.TempDir(), p); !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
		})
	}
	blocker := filepath.Join(t.TempDir(), "file")
	writeFile(t, blocker, []byte("x"))
	if _, err := plan.Write(blocker, newPlan(t, closeStep())); err == nil {
		t.Fatal("a state directory under a file must fail")
	}
}

func TestReadFailures(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if _, err := plan.Read(dir, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing plan: %v", err)
	}
	if _, err := plan.Read(dir, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("empty ref: %v", err)
	}
	p := newPlan(t, closeStep())
	path, err := plan.Write(dir, p)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	writeFile(t, path, []byte(strings.Replace(string(data), "CLOSED", "OPEN", 1)))
	if _, err := plan.Read(dir, p.ID); !errors.Is(err, domain.ErrApplyRefused) {
		t.Fatalf("edited plan must be refused, got %v", err)
	}
	writeFile(t, path, []byte("{broken"))
	if _, err := plan.Read(dir, p.ID); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("corrupt plan: %v", err)
	}
}

func TestListAndPrune(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if plans, err := plan.List(dir); err != nil || plans != nil {
		t.Fatalf("missing dir: %v, %v", plans, err)
	}
	old, recent := newPlan(t, closeStep()), newPlan(t, closeStep())
	old.ID, old.CreatedAt = "20261001T120000Z-old", now.Add(-7*24*time.Hour)
	recent.ID = "20261008T120000Z-new"
	for _, p := range []domain.Plan{old, recent} {
		if _, err := plan.Write(dir, p); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, plan.Path(dir, "broken"), []byte("{"))
	if err := os.Chtimes(plan.Path(dir, "broken"), now.AddDate(-1, 0, 0), now.AddDate(-1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, plan.Path(dir, "fresh-broken"), []byte("{"))
	writeFile(t, filepath.Join(dir, plan.PlansDir, "notes.txt"), []byte("x"))
	plans, err := plan.List(dir)
	if err != nil || len(plans) != 2 || plans[0].ID != recent.ID || plans[1].ID != old.ID {
		t.Fatalf("List = %v, %v", plans, err)
	}
	journal := plan.NewJournal(dir)
	if err := journal.Append(plan.JournalEntry{PlanID: old.ID, Status: plan.StatusDone}); err != nil {
		t.Fatal(err)
	}
	removed, err := plan.Prune(dir, now.Add(-24*time.Hour))
	if err != nil || removed != 3 {
		t.Fatalf("Prune = %d, %v", removed, err)
	}
	if plans, _ := plan.List(dir); len(plans) != 1 || plans[0].ID != recent.ID {
		t.Fatalf("after prune: %v", plans)
	}
	if _, err := plan.Prune(filepath.Join(dir, plan.PlansDir, "notes.txt"), now); err == nil {
		t.Fatal("a plans directory under a file must fail")
	}
}

func TestNewID(t *testing.T) {
	a, err := plan.NewID(now)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := plan.NewID(now)
	if a == b || !strings.HasPrefix(a, now.Format(domain.PlanIDLayout)+"-") {
		t.Fatalf("ids = %q, %q", a, b)
	}
	if _, err := plan.Read(t.TempDir(), a); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a fresh id resolves to nothing: %v", err)
	}
}
