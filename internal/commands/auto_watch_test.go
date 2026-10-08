package commands

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// autoFakeReader serves one project and its item pages.
type autoFakeReader struct {
	project domain.Project
	pages   []domain.ItemPage
	err     error
	calls   int
}

func (r *autoFakeReader) DiscoverProject(context.Context, domain.ProjectRef) (domain.Project, error) {
	return r.project, r.err
}

func (r *autoFakeReader) ListItems(context.Context, domain.Project, domain.ListOptions) (domain.ItemPage, error) {
	if r.calls >= len(r.pages) {
		return domain.ItemPage{}, errors.New("no more pages")
	}
	page := r.pages[r.calls]
	r.calls++
	return page, nil
}

func (r *autoFakeReader) GetItem(context.Context, domain.Project, domain.Reference) (domain.Item, error) {
	return domain.Item{}, domain.ErrNotImplemented
}

func (r *autoFakeReader) Viewer(context.Context) (domain.Viewer, error) {
	return domain.Viewer{}, domain.ErrNotImplemented
}

func (r *autoFakeReader) ViewerPermission(context.Context, string, string) (domain.Permission, error) {
	return "", domain.ErrNotImplemented
}

func (r *autoFakeReader) RepositoryLabels(context.Context, string, string) ([]domain.Label, error) {
	return nil, domain.ErrNotImplemented
}

func (r *autoFakeReader) RepositoryMilestones(context.Context, string, string) ([]domain.Milestone, error) {
	return nil, domain.ErrNotImplemented
}

func (r *autoFakeReader) IssueTypes(context.Context, string) ([]domain.IssueType, error) {
	return nil, domain.ErrNotImplemented
}

func (r *autoFakeReader) IssueComments(context.Context, string) ([]domain.Comment, error) {
	return nil, domain.ErrNotImplemented
}

// autoWatchSeams captures the state writer and serves a fixed diff.
type autoWatchSeams struct {
	diff      alerts.Diff
	diffErr   error
	writeErr  error
	written   *domain.AlertState
	writePath string
}

func autoSetWatchSeams(t *testing.T, s *autoWatchSeams) {
	t.Helper()
	autoDiffStates = func(domain.AlertState, domain.AlertState) (alerts.Diff, error) { return s.diff, s.diffErr }
	autoWriteState = func(path string, state domain.AlertState) error {
		s.writePath, s.written = path, &state
		return s.writeErr
	}
	t.Cleanup(func() {
		autoDiffStates, autoWriteState = alerts.Evaluate, alerts.WriteState
	})
}

func autoWatchEnv(t *testing.T, pages ...domain.ItemPage) (*autoTestEnv, *autoFakeReader) {
	t.Helper()
	env := autoNewTestEnv(t)
	reader := &autoFakeReader{project: domain.Project{Ref: domain.ProjectRef{Owner: "acme", Number: 7}, Title: "Board"}, pages: pages}
	if len(pages) == 0 {
		reader.pages = []domain.ItemPage{{}}
	}
	env.deps.Reader = reader
	return env, reader
}

func TestAutoWatchOnce(t *testing.T) {
	env, reader := autoWatchEnv(t,
		domain.ItemPage{Items: []domain.Item{{ProjectItemID: "a"}}, HasNext: true, NextCursor: "c2"},
		domain.ItemPage{Items: []domain.Item{{ProjectItemID: "b"}}},
	)
	seams := &autoWatchSeams{diff: alerts.Diff{New: []domain.Alert{autoAlert("overdue", domain.SeverityWarning, 3)}, Unchanged: 2}}
	autoSetWatchSeams(t, seams)
	got := autoCaptureNotify(t, nil)
	if err := env.run(t, "--json", "watch", "--once", "--project", "acme/7"); err != nil {
		t.Fatalf("watch: %v\n%s", err, env.errOut.String())
	}
	var report autoWatchReport
	if err := json.Unmarshal(env.out.Bytes(), &report); err != nil {
		t.Fatalf("report: %v\n%s", err, env.out.String())
	}
	if report.New != 1 || report.Notified != 1 || report.Unchanged != 2 || report.Project.Owner != "acme" || report.DryRun {
		t.Fatalf("report = %+v", report)
	}
	if reader.calls != 2 || len(*got) != 1 {
		t.Fatalf("pages read = %d, notified = %d", reader.calls, len(*got))
	}
	want := alerts.StatePath(env.deps.Dirs.State)
	if seams.writePath != want || seams.written == nil || seams.written.GeneratedAt != autoTestNow || seams.written.Project.Number != 7 {
		t.Fatalf("state written to %q: %+v", seams.writePath, seams.written)
	}
	if !autoExists(filepath.Join(env.deps.Dirs.State, autoNotifyLedgerName)) {
		t.Fatal("ledger missing")
	}
	if autoExists(alerts.LockPath(env.deps.Dirs.State)) {
		t.Fatal("lock not released")
	}
}

func TestAutoWatchDryRun(t *testing.T) {
	env, _ := autoWatchEnv(t)
	seams := &autoWatchSeams{diff: alerts.Diff{Escalated: []domain.Alert{autoAlert("wip_exceeded", domain.SeverityCritical, 0)}}}
	autoSetWatchSeams(t, seams)
	got := autoCaptureNotify(t, nil)
	if err := env.run(t, "--dry-run", "watch", "--project", "acme/7"); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 || seams.written != nil {
		t.Fatal("dry run must not notify or write")
	}
	out := env.out.String()
	for _, want := range []string{"dry run", "wip_exceeded", "board", "Escalated"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q:\n%s", want, out)
		}
	}
}

var autoWatchErrorCases = []struct {
	name  string
	args  []string
	seams autoWatchSeams
	code  domain.ExitCode
}{
	{"no project", []string{"watch"}, autoWatchSeams{}, domain.ExitUsage},
	{"bad project", []string{"watch", "--project", "nope"}, autoWatchSeams{}, domain.ExitUsage},
	{"exclusive flags", []string{"watch", "--once", "--install"}, autoWatchSeams{}, domain.ExitUsage},
	{"diff error", []string{"watch", "--project", "acme/7"}, autoWatchSeams{diffErr: errors.New("diff")}, domain.ExitUsage},
	{"write error", []string{"watch", "--project", "acme/7"}, autoWatchSeams{writeErr: errors.New("disk")}, domain.ExitUsage},
	{"bad format", []string{"--format", "xml", "watch", "--project", "acme/7"}, autoWatchSeams{}, domain.ExitUsage},
}

func TestAutoWatchErrors(t *testing.T) {
	for _, tc := range autoWatchErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			env, _ := autoWatchEnv(t)
			seams := tc.seams
			autoSetWatchSeams(t, &seams)
			autoCaptureNotify(t, nil)
			if err := env.run(t, tc.args...); domain.CodeOf(err) != tc.code {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAutoWatchReaderAndStateFailures(t *testing.T) {
	env, reader := autoWatchEnv(t)
	autoSetWatchSeams(t, &autoWatchSeams{})
	reader.err = errors.New("api down")
	if err := env.run(t, "watch", "--project", "acme/7"); err == nil || !strings.Contains(err.Error(), "api down") {
		t.Fatalf("err = %v", err)
	}
	reader.err = nil
	autoWriteTestFile(t, alerts.StatePath(env.deps.Dirs.State), []byte("{broken"))
	if err := env.run(t, "watch", "--project", "acme/7"); err == nil || !strings.Contains(err.Error(), "start over") {
		t.Fatalf("err = %v", err)
	}
	env.deps.Reader = nil
	if err := env.run(t, "watch", "--project", "acme/7"); err == nil || !strings.Contains(err.Error(), "reader") {
		t.Fatalf("err = %v", err)
	}
}

func TestAutoWatchLock(t *testing.T) {
	env, _ := autoWatchEnv(t)
	autoSetWatchSeams(t, &autoWatchSeams{})
	lock := alerts.LockPath(env.deps.Dirs.State)
	autoWriteTestFile(t, lock, []byte(strconv.Itoa(os.Getpid())))
	if err := env.run(t, "watch", "--project", "acme/7"); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("live lock: %v", err)
	}
	autoWriteTestFile(t, lock, []byte("unknown holder"))
	stale := time.Now().Add(-2 * alerts.StaleLockAge)
	if err := os.Chtimes(lock, stale, stale); err != nil {
		t.Fatal(err)
	}
	if err := env.run(t, "watch", "--project", "acme/7"); err != nil {
		t.Fatalf("stale lock: %v", err)
	}
	if autoExists(lock) {
		t.Fatal("lock left behind")
	}
}

func TestAutoWatchScheduler(t *testing.T) {
	env, _ := autoWatchEnv(t)
	var installed, removed []alerts.SchedulerOptions
	autoInstallScheduler = func(_ context.Context, o alerts.SchedulerOptions) error { installed = append(installed, o); return nil }
	autoUninstallScheduler = func(_ context.Context, o alerts.SchedulerOptions) error { removed = append(removed, o); return nil }
	t.Cleanup(func() {
		autoInstallScheduler, autoUninstallScheduler = alerts.InstallScheduler, alerts.UninstallScheduler
	})
	if err := env.run(t, "--dry-run", "watch", "--install", "--project", "acme/7"); err != nil {
		t.Fatal(err)
	}
	if err := env.run(t, "watch", "--install", "--interval", "30m", "--project", "acme/7"); err != nil {
		t.Fatal(err)
	}
	if len(installed) != 2 || !installed[0].DryRun || installed[0].Interval != alerts.DefaultInterval || installed[1].Interval != 30*time.Minute {
		t.Fatalf("installed = %+v", installed)
	}
	if installed[0].Binary == "" || installed[0].Project.Number != 7 || installed[0].Out == nil || installed[0].Home != env.home {
		t.Fatalf("options = %+v", installed[0])
	}
	if err := env.run(t, "watch", "--uninstall", "--project", "acme/7"); err != nil || len(removed) != 1 {
		t.Fatalf("uninstall: %v, %d", err, len(removed))
	}
	if err := env.run(t, "watch", "--install", "--interval", "30s", "--project", "acme/7"); domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("short interval: %v", err)
	}
	if err := env.run(t, "watch", "--install"); domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("no project: %v", err)
	}
}

func TestAutoWatchHelpers(t *testing.T) {
	if autoAlertItem(domain.ItemRef{}) != "board" || autoAlertItem(domain.ItemRef{Repository: "o/r", Number: 4}) != "o/r#4" {
		t.Fatal("autoAlertItem")
	}
	cfg := &domain.Config{Project: domain.ProjectRef{Owner: "acme", Number: 7}}
	state := autoEvaluateState(cfg, domain.Project{}, nil, autoTestNow, domain.AlertState{})
	if state.Alerts == nil || len(state.Alerts) != 0 || state.Project.Number != 7 || state.Summary == "" {
		t.Fatalf("state = %+v", state)
	}
	deps := &Deps{Out: &strings.Builder{}, Flags: GlobalFlags{Format: "md"}}
	if err := autoRender(deps, autoWatchDocument(autoWatchReport{})); err != nil {
		t.Fatal(err)
	}
	if (&Deps{}).Now().IsZero() {
		t.Fatal("Now without a clock")
	}
	autoWarn(&Deps{}, "no stderr %d", 1)
}
