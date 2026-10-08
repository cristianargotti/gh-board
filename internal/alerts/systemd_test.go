package alerts_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/alerts"
)

func TestSystemdInstallAndUninstall(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, systemdPaths)
	if err := alerts.InstallScheduler(ctx, h.options("linux", false)); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	service, timer := h.unit(".service"), h.unit(".timer")
	data, err := os.ReadFile(service)
	if err != nil || !strings.Contains(string(data), `Environment="GH_PATH=/usr/bin/gh"`) || !strings.Contains(string(data), "ExecStart=") {
		t.Fatalf("service = %q, %v", data, err)
	}
	if data, err := os.ReadFile(timer); err != nil || !strings.Contains(string(data), "OnUnitActiveSec=10min") {
		t.Fatalf("timer = %q, %v", data, err)
	}
	assertPerm(t, service)
	assertPerm(t, timer)
	assertLines(t, h.runner.lines(), "systemctl --user daemon-reload\nsystemctl --user enable --now gh-board.watch.timer")
	assertSystemdUninstall(t, h)
}

// assertSystemdUninstall removes the installed units and checks that a
// second uninstall finds nothing.
func assertSystemdUninstall(t *testing.T, h *harness) {
	t.Helper()
	ctx := context.Background()
	service, timer := h.unit(".service"), h.unit(".timer")
	h.runner.calls = nil
	h.out.Reset()
	if err := alerts.UninstallScheduler(ctx, h.options("linux", false)); err != nil {
		t.Fatalf("UninstallScheduler: %v", err)
	}
	for _, path := range []string{service, timer} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("uninstall must remove %s", path)
		}
	}
	assertLines(t, h.runner.lines(), "systemctl --user disable --now gh-board.watch.timer\nsystemctl --user daemon-reload\ncrontab -l")
	if out := h.out.String(); !strings.Contains(out, "removed "+timer+"\nremoved "+service+"\n") {
		t.Fatalf("output = %q", out)
	}
	h.out.Reset()
	if err := alerts.UninstallScheduler(ctx, h.options("linux", false)); err != nil || h.out.String() != "nothing to remove\n" {
		t.Fatalf("second uninstall: %v, %q", err, h.out.String())
	}
}

// systemctlFailing fails the named systemctl verb and nothing else.
func systemctlFailing(verb string) func(alerts.Command) ([]byte, error) {
	return func(cmd alerts.Command) ([]byte, error) {
		if cmd.Name == "systemctl" && cmd.Args[1] == verb {
			return []byte("Failed to connect to bus"), errors.New("systemctl: exit status 1")
		}
		return nil, nil
	}
}

var systemdFailureCases = []struct {
	verb string
	text string
}{
	{verb: "daemon-reload", text: "scheduler: systemctl"},
	{verb: "enable", text: "scheduler: systemctl"},
}

func TestSystemdInstallFailures(t *testing.T) {
	for _, tc := range systemdFailureCases {
		t.Run(tc.verb, func(t *testing.T) {
			h := newHarness(t, systemdPaths)
			h.runner.respond = systemctlFailing(tc.verb)
			err := alerts.InstallScheduler(context.Background(), h.options("linux", false))
			if err == nil || !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("error = %v, want %q", err, tc.text)
			}
		})
	}
	h := newHarness(t, systemdPaths)
	opts := h.options("linux", false)
	opts.Home = h.bin
	if err := alerts.InstallScheduler(context.Background(), opts); err == nil {
		t.Fatal("units under a file cannot be written")
	}
}

func TestSystemdUninstallTolerates(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, systemdPaths)
	if err := alerts.InstallScheduler(ctx, h.options("linux", false)); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	h.runner.calls = nil
	h.runner.respond = systemctlFailing("disable")
	if err := alerts.UninstallScheduler(ctx, h.options("linux", false)); err != nil {
		t.Fatalf("a timer that is not active still gets its files removed: %v", err)
	}
	if _, err := os.Stat(h.unit(".timer")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("uninstall must remove the timer")
	}
	h.runner.respond = systemctlFailing("daemon-reload")
	if err := alerts.InstallScheduler(ctx, h.options("linux", false)); err == nil {
		t.Fatal("install needs the reload")
	}
	if err := alerts.UninstallScheduler(ctx, h.options("linux", false)); err == nil || !strings.Contains(err.Error(), "scheduler: systemctl") {
		t.Fatalf("a failed reload after the removal is reported: %v", err)
	}
}
