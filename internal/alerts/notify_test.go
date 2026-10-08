package alerts_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// The commands each notifier runs for the alert "late", as the dry run
// prints them; <script> stands for the toast script under the directory.
const (
	osascriptLine  = `/usr/bin/osascript -e "on run argv" -e "display notification (item 2 of argv) with title (item 1 of argv)" -e "end run" "gh board" "Atrasada há 3 dias (acme/team-docs#1)"`
	notifySendLine = `/usr/bin/notify-send "--app-name=gh board" --urgency=normal -- "gh board" "Atrasada há 3 dias (acme/team-docs#1)"`
	powershellLine = `C:\ps\powershell.exe -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File <script> -Title "gh board" -Body "Atrasada há 3 dias (acme/team-docs#1)"`
	wipLine        = "[critical] WIP excedido: Em andamento 5/4\n"
)

var (
	osascriptPath  = map[string]string{"osascript": "/usr/bin/osascript"}
	notifySendPath = map[string]string{"notify-send": "/usr/bin/notify-send"}
	powershellPath = map[string]string{"powershell": `C:\ps\powershell.exe`}
)

type notifyCase struct {
	name   string
	goos   string
	paths  map[string]string
	noDir  bool
	alerts []domain.Alert
	calls  []string
	out    string
	err    string
}

var notifyCases = []notifyCase{
	{name: "macOS through osascript", goos: "darwin", paths: osascriptPath, alerts: []domain.Alert{late}, calls: []string{osascriptLine}},
	{name: "Linux through notify-send", goos: "linux", paths: notifySendPath, alerts: []domain.Alert{late}, calls: []string{notifySendLine}},
	{name: "Windows through the toast script", goos: "windows", paths: powershellPath, alerts: []domain.Alert{late}, calls: []string{powershellLine}},
	{name: "another unix through notify-send", goos: "freebsd", paths: notifySendPath, alerts: []domain.Alert{late}, calls: []string{notifySendLine}},
	{name: "stdout without a notifier, most severe first", goos: "linux", alerts: []domain.Alert{late, wip}, out: wipLine + "[warning] Atrasada há 3 dias (acme/team-docs#1)\n"},
	{name: "stdout without powershell", goos: "windows", alerts: []domain.Alert{wip}, out: wipLine},
	{name: "nothing to notify", goos: "darwin", paths: osascriptPath},
	{name: "windows needs a directory", goos: "windows", paths: powershellPath, noDir: true, alerts: []domain.Alert{late}, err: "directory for the toast script"},
}

func TestNotify(t *testing.T) {
	for _, tc := range notifyCases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths}
			out := &bytes.Buffer{}
			opts := alerts.NotifyOptions{GOOS: tc.goos, Out: out, Runner: runner, Dir: t.TempDir()}
			if tc.noDir {
				opts.Dir = ""
			}
			err := alerts.Notify(context.Background(), tc.alerts, opts)
			if wantError(t, err, tc.err) {
				return
			}
			script := filepath.Join(opts.Dir, "toast.ps1")
			assertLines(t, runner.lines(), strings.ReplaceAll(strings.Join(tc.calls, "\n"), "<script>", script))
			assertOutput(t, out, tc.out)
		})
	}
}

func TestNotifyWritesTheToastScript(t *testing.T) {
	dir := t.TempDir()
	opts := alerts.NotifyOptions{GOOS: "windows", Out: &bytes.Buffer{}, Runner: &fakeRunner{paths: powershellPath}, Dir: filepath.Join(dir, "state")}
	if err := alerts.Notify(context.Background(), []domain.Alert{late}, opts); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	script := filepath.Join(opts.Dir, "toast.ps1")
	data, err := os.ReadFile(script)
	if err != nil || !strings.HasPrefix(string(data), "param([string]$Title, [string]$Body)\n") {
		t.Fatalf("script = %q, %v", data, err)
	}
	if !strings.Contains(string(data), "CreateToastNotifier") || strings.Contains(string(data), "Atrasada") {
		t.Fatal("the script is fixed and never carries alert text")
	}
	assertPerm(t, script)
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts.Dir = filepath.Join(blocker, "under")
	if err := alerts.Notify(context.Background(), []domain.Alert{late}, opts); err == nil {
		t.Fatal("a script that cannot be written must fail")
	}
}

func TestNotifyFailures(t *testing.T) {
	runner := &fakeRunner{paths: osascriptPath, respond: failing("display notification: boom")}
	err := alerts.Notify(context.Background(), []domain.Alert{late}, alerts.NotifyOptions{GOOS: "darwin", Out: &bytes.Buffer{}, Runner: runner})
	if err == nil || !strings.HasPrefix(err.Error(), "notify: ") {
		t.Fatalf("error = %v", err)
	}
	if err := alerts.Notify(context.Background(), nil, alerts.NotifyOptions{}); err != nil {
		t.Fatalf("defaults with nothing to notify: %v", err)
	}
}

var notificationCases = []struct {
	name   string
	alerts []domain.Alert
	bodies []string
	levels []domain.Severity
}{
	{
		name:   "most severe first, cap with a summary",
		alerts: []domain.Alert{late, blocked, wip, lateNow, alert("x", domain.SeverityInfo, 5, "a"), alert("x", domain.SeverityInfo, 6, "b"), alert("x", domain.SeverityCritical, 7, "c")},
		bodies: []string{"WIP excedido: Em andamento 5/4", "Atrasada há 7 dias (acme/team-docs#1)", "c (acme/team-docs#7)", "Atrasada há 3 dias (acme/team-docs#1)", "Mais 3 alertas em alerts.json: veja gh board attention"},
		levels: []domain.Severity{domain.SeverityCritical, domain.SeverityCritical, domain.SeverityCritical, domain.SeverityWarning, domain.SeverityWarning},
	},
	{
		name:   "exactly the cap needs no summary",
		alerts: []domain.Alert{late, late, late, late, late},
		bodies: []string{"Atrasada há 3 dias (acme/team-docs#1)", "Atrasada há 3 dias (acme/team-docs#1)", "Atrasada há 3 dias (acme/team-docs#1)", "Atrasada há 3 dias (acme/team-docs#1)", "Atrasada há 3 dias (acme/team-docs#1)"},
		levels: []domain.Severity{domain.SeverityWarning, domain.SeverityWarning, domain.SeverityWarning, domain.SeverityWarning, domain.SeverityWarning},
	},
	{
		name:   "a leading dash is dropped and a board alert has no reference",
		alerts: []domain.Alert{alert("x", domain.SeverityInfo, 3, "- cuidado"), wip},
		bodies: []string{"WIP excedido: Em andamento 5/4", "cuidado (acme/team-docs#3)"},
		levels: []domain.Severity{domain.SeverityCritical, domain.SeverityInfo},
	},
	{name: "nothing"},
}

func TestNotifications(t *testing.T) {
	for _, tc := range notificationCases {
		t.Run(tc.name, func(t *testing.T) {
			got := alerts.Notifications(tc.alerts)
			if len(got) != len(tc.bodies) {
				t.Fatalf("got %d notifications, want %d: %+v", len(got), len(tc.bodies), got)
			}
			for i, n := range got {
				if n.Title != "gh board" || n.Body != tc.bodies[i] || n.Severity != tc.levels[i] {
					t.Errorf("notification %d = %+v, want %q %s", i, n, tc.bodies[i], tc.levels[i])
				}
			}
		})
	}
}
