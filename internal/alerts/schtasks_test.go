package alerts_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
)

var schtasksScheduleCases = []struct {
	interval time.Duration
	want     string
}{
	{interval: 10 * time.Minute, want: "/SC MINUTE /MO 10"},
	{interval: 90 * time.Minute, want: "/SC MINUTE /MO 90"},
	{interval: 2 * time.Hour, want: "/SC HOURLY /MO 2"},
	{interval: 24 * time.Hour, want: "/SC DAILY /MO 1"},
}

func TestSchtasksInstall(t *testing.T) {
	for _, tc := range schtasksScheduleCases {
		t.Run(tc.interval.String(), func(t *testing.T) {
			h := newHarness(t, noPaths)
			opts := h.options("windows", false)
			opts.Interval, opts.Home = tc.interval, ""
			if err := alerts.InstallScheduler(context.Background(), opts); err != nil {
				t.Fatalf("InstallScheduler: %v", err)
			}
			want := "schtasks /Create /F " + tc.want + " /TN gh-board.watch /TR " + strconv.Quote(`"`+h.bin+`" watch --once --project acme/7`)
			if got := h.runner.lines(); len(got) != 1 || got[0] != want {
				t.Fatalf("commands = %v\nwant %s", got, want)
			}
			if h.out.String() != "ran "+want+"\n" {
				t.Fatalf("output = %q", h.out.String())
			}
		})
	}
}

// The console host, when the machine has it, runs the task without a
// visible window; the dry run shows the exact line.
func TestSchtasksInstallHeadless(t *testing.T) {
	host := `C:\Windows\System32\conhost.exe`
	h := newHarness(t, map[string]string{"conhost.exe": host})
	opts := h.options("windows", true)
	opts.Home = ""
	if err := alerts.InstallScheduler(context.Background(), opts); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	want := "would run schtasks /Create /F /SC MINUTE /MO 10 /TN gh-board.watch /TR " +
		strconv.Quote(`"`+host+`" --headless "`+h.bin+`" watch --once --project acme/7`) + "\n"
	if h.out.String() != want {
		t.Fatalf("output = %q\nwant %q", h.out.String(), want)
	}
	if len(h.runner.calls) != 0 {
		t.Fatalf("a dry run must not run schtasks: %v", h.runner.lines())
	}
}

// schtasksAnswering fails the /Query when absent is set and the /Delete
// when stuck is set.
func schtasksAnswering(absent, stuck bool) func(alerts.Command) ([]byte, error) {
	return func(cmd alerts.Command) ([]byte, error) {
		switch cmd.Args[0] {
		case "/Query":
			if absent {
				return []byte("ERROR: The system cannot find the file specified."), errors.New("schtasks: exit status 1")
			}
		case "/Delete":
			if stuck {
				return []byte("ERROR: Access is denied."), errors.New("schtasks: exit status 1")
			}
		}
		return nil, nil
	}
}

var schtasksUninstallCases = []struct {
	name   string
	absent bool
	stuck  bool
	dryRun bool
	calls  string
	out    string
	err    string
}{
	{name: "an absent task", absent: true, calls: "schtasks /Query /TN gh-board.watch", out: "nothing to remove: task gh-board.watch is absent\n"},
	{name: "a present task", calls: "schtasks /Query /TN gh-board.watch\nschtasks /Delete /F /TN gh-board.watch", out: "ran schtasks /Delete /F /TN gh-board.watch\n"},
	{name: "a dry run only queries", dryRun: true, calls: "schtasks /Query /TN gh-board.watch", out: "would run schtasks /Delete /F /TN gh-board.watch\n"},
	{name: "a refused delete", stuck: true, err: "scheduler: schtasks"},
}

func TestSchtasksUninstall(t *testing.T) {
	for _, tc := range schtasksUninstallCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, noPaths)
			h.runner.respond = schtasksAnswering(tc.absent, tc.stuck)
			opts := h.options("windows", tc.dryRun)
			opts.Home = ""
			err := alerts.UninstallScheduler(context.Background(), opts)
			if wantError(t, err, tc.err) {
				return
			}
			assertLines(t, h.runner.lines(), tc.calls)
			assertOutput(t, h.out, tc.out)
		})
	}
}
