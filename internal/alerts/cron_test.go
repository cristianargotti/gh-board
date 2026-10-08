package alerts_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

const ownedLine = "*/5 * * * * /old/gh-board watch --once --project acme/7 # gh-board.watch"

func TestCronInstall(t *testing.T) {
	h := newHarness(t, cronPaths)
	h.runner.respond = answerCrontab(existingCrontab + ownedLine + "\n")
	if err := alerts.InstallScheduler(context.Background(), h.options("linux", false)); err != nil {
		t.Fatalf("InstallScheduler: %v", err)
	}
	if got := h.runner.lines(); strings.Join(got, "\n") != "crontab -l\ncrontab -" {
		t.Fatalf("commands = %v", got)
	}
	want := existingCrontab + "*/10 * * * * GH_PATH=/usr/bin/gh " + h.cronBin() + " watch --once --project acme/7 # gh-board.watch\n"
	if got := string(h.runner.calls[1].Stdin); got != want {
		t.Fatalf("crontab written:\n%s\nwant:\n%s", got, want)
	}
	if out := h.out.String(); out != "systemd is absent: installing a cron line\nran crontab -\n" {
		t.Fatalf("output = %q", out)
	}
}

type cronUninstallCase struct {
	name    string
	crontab string
	fail    string
	calls   string
	stdin   string
	out     string
	err     string
}

var cronUninstallCases = []cronUninstallCase{
	{name: "no crontab at all", fail: "no crontab for someone", calls: "crontab -l", out: "nothing to remove\n"},
	{name: "a crontab without the line", crontab: existingCrontab, calls: "crontab -l", out: "nothing to remove\n"},
	{name: "only the line goes with the crontab", crontab: ownedLine + "\n", calls: "crontab -l\ncrontab -r", out: "ran crontab -r\n"},
	{name: "the other lines stay", crontab: ownedLine + "\n" + existingCrontab, calls: "crontab -l\ncrontab -", stdin: existingCrontab, out: "ran crontab -\n"},
	{name: "another failure is reported", fail: "you are not allowed to use this program", calls: "crontab -l", err: "scheduler: crontab"},
}

func TestCronUninstall(t *testing.T) {
	for _, tc := range cronUninstallCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, cronPaths)
			h.runner.respond = answerCrontab(tc.crontab)
			if tc.fail != "" {
				h.runner.respond = failing(tc.fail)
			}
			err := alerts.UninstallScheduler(context.Background(), h.options("linux", false))
			if wantError(t, err, tc.err) {
				return
			}
			assertLines(t, h.runner.lines(), tc.calls)
			assertOutput(t, h.out, tc.out)
			if last := h.runner.calls[len(h.runner.calls)-1]; string(last.Stdin) != tc.stdin {
				t.Fatalf("crontab written = %q, want %q", last.Stdin, tc.stdin)
			}
		})
	}
}

var cronScheduleCases = []struct {
	interval time.Duration
	want     string
	err      bool
}{
	{interval: 10 * time.Minute, want: "*/10 * * * * "},
	{interval: 59 * time.Minute, want: "*/59 * * * * "},
	{interval: time.Hour, want: "0 */1 * * * "},
	{interval: 6 * time.Hour, want: "0 */6 * * * "},
	{interval: 24 * time.Hour, want: "0 0 * * * "},
	{interval: 90 * time.Minute, err: true},
}

func TestCronSchedule(t *testing.T) {
	for _, tc := range cronScheduleCases {
		t.Run(tc.interval.String(), func(t *testing.T) {
			h := newHarness(t, cronPaths)
			h.runner.respond = failing("no crontab for someone")
			opts := h.options("linux", true)
			opts.Interval = tc.interval
			err := alerts.InstallScheduler(context.Background(), opts)
			if tc.err {
				if !errors.Is(err, domain.ErrUsage) || !strings.Contains(err.Error(), "whole hours") {
					t.Fatalf("error = %v, want a usage error", err)
				}
				return
			}
			if err != nil || !strings.Contains(h.out.String(), "\n"+tc.want) {
				t.Fatalf("output = %q, %v; want the schedule %q", h.out.String(), err, tc.want)
			}
		})
	}
}

func TestCronInstallFailures(t *testing.T) {
	h := newHarness(t, cronPaths)
	h.runner.respond = failing("you are not allowed to use this program")
	err := alerts.InstallScheduler(context.Background(), h.options("linux", false))
	if err == nil || !strings.Contains(err.Error(), "scheduler: crontab") {
		t.Fatalf("error = %v", err)
	}
	h = newHarness(t, cronPaths)
	h.runner.respond = func(cmd alerts.Command) ([]byte, error) {
		if cmd.Args[0] == "-" {
			return nil, errors.New("crontab: installing new crontab failed")
		}
		return nil, nil
	}
	err = alerts.InstallScheduler(context.Background(), h.options("linux", false))
	if err == nil || !strings.Contains(err.Error(), "installing new crontab") {
		t.Fatalf("error = %v", err)
	}
}
