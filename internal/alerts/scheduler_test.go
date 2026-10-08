package alerts_test

import (
	"bytes"
	"context"
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

// The programs each scheduler finds on the PATH in the tests.
var (
	darwinPaths  = map[string]string{"gh": "/opt/homebrew/bin/gh"}
	systemdPaths = map[string]string{"systemctl": "/usr/bin/systemctl", "crontab": "/usr/bin/crontab", "gh": "/usr/bin/gh"}
	cronPaths    = map[string]string{"crontab": "/usr/bin/crontab", "gh": "/usr/bin/gh"}
	noPaths      = map[string]string{}
)

// existingCrontab is what crontab -l answers in the tests.
const existingCrontab = "MAILTO=someone\n0 7 * * 1 /usr/local/bin/backup --weekly\n"

// harness holds a temporary home, a fake binary and a recording runner.
type harness struct {
	home   string
	bin    string
	out    *bytes.Buffer
	runner *fakeRunner
}

func newHarness(t *testing.T, paths map[string]string) *harness {
	t.Helper()
	home := resolvedHome(t)
	return &harness{home: home, bin: fakeBinary(t, home), out: &bytes.Buffer{}, runner: &fakeRunner{paths: paths}}
}

func (h *harness) options(goos string, dryRun bool) alerts.SchedulerOptions {
	return alerts.SchedulerOptions{
		GOOS: goos, Binary: h.bin, Project: fxProject, Interval: 10 * time.Minute, DryRun: dryRun,
		Out: h.out, Home: h.home, UID: 501, Runner: h.runner,
	}
}

func (h *harness) plist() string {
	return filepath.Join(h.home, "Library", "LaunchAgents", "gh-board.watch.plist")
}

func (h *harness) unit(ext string) string {
	return filepath.Join(h.home, ".config", "systemd", "user", "gh-board.watch"+ext)
}

// cronBin is the binary as the cron line quotes it: as is when the shell
// takes it, in single quotes when it carries a backslash (Windows paths).
func (h *harness) cronBin() string {
	if strings.ContainsAny(h.bin, `\ ~`) {
		return "'" + h.bin + "'"
	}
	return h.bin
}

// golden reads a dry-run expectation and fills the path placeholders.
func (h *harness) golden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	r := strings.NewReplacer(
		"{{plist}}", h.plist(), "{{log}}", filepath.Join(h.home, "Library", "Logs", "gh-board.watch.log"),
		"{{service}}", h.unit(".service"), "{{timer}}", h.unit(".timer"),
		"{{bin}}", h.bin, "{{binq}}", strings.ReplaceAll(h.bin, `\`, `\\`), "{{binsh}}", h.cronBin(),
	)
	return r.Replace(strings.ReplaceAll(string(data), "\r\n", "\n"))
}

// answerCrontab makes crontab -l answer the content and every other
// command succeed.
func answerCrontab(content string) func(alerts.Command) ([]byte, error) {
	return func(cmd alerts.Command) ([]byte, error) {
		if cmd.Name == "crontab" && cmd.Args[0] == "-l" {
			return []byte(content), nil
		}
		return nil, nil
	}
}

var dryRunCases = []struct {
	name   string
	goos   string
	paths  map[string]string
	golden string
	calls  []string
}{
	{name: "launchd agent", goos: "darwin", paths: darwinPaths, golden: "install-darwin.txt"},
	{name: "systemd user timer", goos: "linux", paths: systemdPaths, golden: "install-systemd.txt"},
	{name: "cron line without systemd", goos: "linux", paths: cronPaths, golden: "install-cron.txt", calls: []string{"crontab -l"}},
}

func TestInstallDryRun(t *testing.T) {
	for _, tc := range dryRunCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.paths)
			h.runner.respond = answerCrontab(existingCrontab)
			if err := alerts.InstallScheduler(context.Background(), h.options(tc.goos, true)); err != nil {
				t.Fatalf("InstallScheduler: %v", err)
			}
			assertOutput(t, h.out, h.golden(t, tc.golden))
			assertLines(t, h.runner.lines(), strings.Join(tc.calls, "\n"))
			h.assertNothingWritten(t)
		})
	}
}

// assertNothingWritten checks that no scheduler file exists under the home.
func (h *harness) assertNothingWritten(t *testing.T) {
	t.Helper()
	for _, path := range []string{h.plist(), h.unit(".service"), h.unit(".timer")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a dry run must write nothing, found %s", path)
		}
	}
}

func TestInstallDryRunWindows(t *testing.T) {
	h := newHarness(t, noPaths)
	opts := h.options("windows", true)
	opts.Home = ""
	if err := alerts.InstallScheduler(context.Background(), opts); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	want := "would run schtasks /Create /F /SC MINUTE /MO 10 /TN gh-board.watch /TR " +
		strconv.Quote(`"`+h.bin+`" watch --once --project acme/7`) + "\n"
	if h.out.String() != want || len(h.runner.calls) != 0 {
		t.Fatalf("output = %q, calls = %v\nwant %q", h.out.String(), h.runner.lines(), want)
	}
}

// The subtest names stay clear of the absent text, because the temporary
// home carries the test name and the output prints the home.
var withoutGHCases = []struct {
	name   string
	goos   string
	paths  map[string]string
	absent string
}{
	{name: "launchd", goos: "darwin", paths: noPaths, absent: "EnvironmentVariables"},
	{name: "systemd", goos: "linux", paths: map[string]string{"systemctl": "/usr/bin/systemctl"}, absent: "Environment="},
	{name: "cron", goos: "linux", paths: map[string]string{"crontab": "/usr/bin/crontab"}, absent: "GH_PATH"},
}

func TestInstallWithoutGH(t *testing.T) {
	for _, tc := range withoutGHCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.paths)
			h.runner.respond = answerCrontab("")
			if err := alerts.InstallScheduler(context.Background(), h.options(tc.goos, true)); err != nil {
				t.Fatalf("InstallScheduler: %v", err)
			}
			if strings.Contains(h.out.String(), tc.absent) || !strings.Contains(h.out.String(), "watch") {
				t.Fatalf("output:\n%s", h.out.String())
			}
		})
	}
}

type optionsCase struct {
	name   string
	change func(o *alerts.SchedulerOptions)
	err    error
	text   string
}

var optionsCases = []optionsCase{
	{name: "an unknown system has no scheduler", change: func(o *alerts.SchedulerOptions) { o.GOOS = "plan9" }, err: domain.ErrUsage, text: "no scheduler for plan9"},
	{name: "the home is required outside windows", change: func(o *alerts.SchedulerOptions) { o.Home = "" }, err: domain.ErrUsage, text: "home directory"},
	{name: "a project is required", change: func(o *alerts.SchedulerOptions) { o.Project = domain.ProjectRef{} }, err: domain.ErrUsage, text: "project is required"},
	{name: "the interval is checked", change: func(o *alerts.SchedulerOptions) { o.Interval = 90 * time.Second }, err: domain.ErrUsage, text: "whole minutes"},
	{name: "the executable must exist", change: func(o *alerts.SchedulerOptions) { o.Binary = filepath.Join(o.Home, "missing") }, text: "executable"},
	{name: "the home must be writable", change: func(o *alerts.SchedulerOptions) { o.Home = o.Binary }, text: "scheduler: "},
}

func TestInstallOptions(t *testing.T) {
	for _, tc := range optionsCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, darwinPaths)
			opts := h.options("darwin", false)
			tc.change(&opts)
			err := alerts.InstallScheduler(context.Background(), opts)
			if err == nil || !strings.Contains(err.Error(), tc.text) || (tc.err != nil && !errors.Is(err, tc.err)) {
				t.Fatalf("error = %v, want %q wrapping %v", err, tc.text, tc.err)
			}
			if len(h.runner.calls) != 0 {
				t.Fatalf("nothing may run after a refused install, ran %v", h.runner.lines())
			}
		})
	}
}

func TestUninstallOptions(t *testing.T) {
	h := newHarness(t, noPaths)
	opts := h.options("plan9", false)
	if err := alerts.UninstallScheduler(context.Background(), opts); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("error = %v, want a usage error", err)
	}
	opts.GOOS, opts.Home = "linux", ""
	if err := alerts.UninstallScheduler(context.Background(), opts); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("error = %v, want a usage error", err)
	}
}

// TestInstallDefaults covers the zero options: the running binary, the
// default interval, the current user and the real runner, which a dry run
// never launches.
func TestInstallDefaults(t *testing.T) {
	h := newHarness(t, noPaths)
	opts := alerts.SchedulerOptions{GOOS: "darwin", Project: fxProject, DryRun: true, Out: h.out, Home: h.home}
	if err := alerts.InstallScheduler(context.Background(), opts); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	out := h.out.String()
	for _, want := range []string{"<string>" + exe + "</string>", "<integer>600</integer>", "gui/" + strconv.Itoa(os.Getuid()) + "/gh-board.watch"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
