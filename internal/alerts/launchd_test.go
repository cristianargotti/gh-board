package alerts_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/alerts"
)

func TestLaunchdInstallAndUninstall(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, darwinPaths)
	if err := alerts.InstallScheduler(ctx, h.options("darwin", false)); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	plist := h.plist()
	data, err := os.ReadFile(plist)
	if err != nil || !strings.Contains(string(data), "<string>"+h.bin+"</string>") || !strings.Contains(string(data), "GH_PATH") {
		t.Fatalf("plist = %q, %v", data, err)
	}
	assertPerm(t, plist)
	want := []string{"launchctl bootout gui/501/gh-board.watch", "launchctl bootstrap gui/501 " + plist}
	if got := h.runner.lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	if out := h.out.String(); out != "wrote "+plist+"\nran "+want[0]+"\nran "+want[1]+"\n" {
		t.Fatalf("output = %q", out)
	}
	h.runner.calls = nil
	h.out.Reset()
	if err := alerts.UninstallScheduler(ctx, h.options("darwin", false)); err != nil {
		t.Fatalf("UninstallScheduler: %v", err)
	}
	if _, err := os.Stat(plist); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("uninstall must remove the plist")
	}
	if got := h.runner.lines(); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("uninstall commands = %v", got)
	}
	if out := h.out.String(); out != "ran "+want[0]+"\nremoved "+plist+"\n" {
		t.Fatalf("output = %q", out)
	}
	h.out.Reset()
	if err := alerts.UninstallScheduler(ctx, h.options("darwin", false)); err != nil || h.out.String() != "nothing to remove: "+plist+" is absent\n" {
		t.Fatalf("second uninstall: %v, %q", err, h.out.String())
	}
}

func TestLaunchdUninstallDryRun(t *testing.T) {
	h := newHarness(t, darwinPaths)
	if err := os.MkdirAll(h.plist(), 0o700); err != nil { // a directory stands in for the agent file
		t.Fatal(err)
	}
	if err := alerts.UninstallScheduler(context.Background(), h.options("darwin", true)); err != nil {
		t.Fatalf("UninstallScheduler: %v", err)
	}
	want := "would run launchctl bootout gui/501/gh-board.watch\nwould remove " + h.plist() + "\n"
	if h.out.String() != want || len(h.runner.calls) != 0 {
		t.Fatalf("output = %q, calls = %v", h.out.String(), h.runner.lines())
	}
}

// launchctlFailing fails the named launchctl verb and nothing else.
func launchctlFailing(verb string) func(alerts.Command) ([]byte, error) {
	return func(cmd alerts.Command) ([]byte, error) {
		if cmd.Args[0] == verb {
			return []byte("Boot-out failed: 3: No such process"), errors.New("launchctl: exit status 3")
		}
		return nil, nil
	}
}

func TestLaunchdFailures(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, darwinPaths)
	h.runner.respond = launchctlFailing("bootout")
	if err := alerts.InstallScheduler(ctx, h.options("darwin", false)); err != nil {
		t.Fatalf("a job that was not loaded must not fail the install: %v", err)
	}
	if strings.Contains(h.out.String(), "ran launchctl bootout") {
		t.Fatalf("a failed bootout is not reported as run:\n%s", h.out.String())
	}
	h.runner.respond = launchctlFailing("bootstrap")
	err := alerts.InstallScheduler(ctx, h.options("darwin", false))
	if err == nil || !strings.Contains(err.Error(), "scheduler: launchctl") {
		t.Fatalf("error = %v", err)
	}
	h.runner.respond = launchctlFailing("bootout")
	if err := alerts.UninstallScheduler(ctx, h.options("darwin", false)); err != nil {
		t.Fatalf("an agent that is not loaded still gets its file removed: %v", err)
	}
	if _, err := os.Stat(h.plist()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("uninstall must remove the plist")
	}
}
