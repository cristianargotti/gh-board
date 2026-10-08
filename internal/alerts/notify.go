package alerts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// Operating systems with a native notifier and scheduler of their own.
const (
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"
)

// notifyTitle heads every notification.
const notifyTitle = "gh board"

// NotifyOptions select the notifier. GOOS defaults to runtime.GOOS; Out
// receives the text when no notifier exists on the system, stdout when
// nil; Runner launches the notifier, ExecRunner when nil; Dir keeps the
// toast script Windows needs, normally the state directory.
type NotifyOptions struct {
	GOOS   string
	Out    io.Writer
	Runner Runner
	Dir    string
}

// withDefaults fills the zero options.
func (o NotifyOptions) withDefaults() NotifyOptions {
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.Out == nil {
		o.Out = os.Stdout
	}
	if o.Runner == nil {
		o.Runner = ExecRunner{}
	}
	return o
}

// Notification is the text of one toast: the title names the kit, the
// body is the PT-BR message followed by the item reference.
type Notification struct {
	Title    string
	Body     string
	Severity domain.Severity
}

// sender delivers one notification.
type sender func(ctx context.Context, n Notification) error

// Notify sends one notification per alert through osascript (macOS),
// notify-send (Linux) or a PowerShell toast (Windows), passing the text as
// arguments and never through a script. The caller passes the new and the
// escalated alerts; Notifications orders and caps them. Without a notifier
// on the PATH the text goes to Out, one line per alert.
func Notify(ctx context.Context, alerts []domain.Alert, opts NotifyOptions) error {
	opts = opts.withDefaults()
	notes := Notifications(alerts)
	if len(notes) == 0 {
		return nil
	}
	send, err := notifier(opts)
	if err != nil {
		return err
	}
	for _, n := range notes {
		if err := send(ctx, n); err != nil {
			return fmt.Errorf("notify: %w", err)
		}
	}
	return nil
}

// Notifications turns alerts into notification texts, most severe first,
// capped at MaxToastsPerHour: when more alerts arrive, the last text counts
// the rest, which stay in alerts.json.
func Notifications(alerts []domain.Alert) []Notification {
	sorted := make([]domain.Alert, len(alerts))
	copy(sorted, alerts)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Severity.Rank() > sorted[j].Severity.Rank()
	})
	keep := len(sorted)
	if keep > MaxToastsPerHour {
		keep = MaxToastsPerHour - 1
	}
	out := make([]Notification, 0, keep+1)
	for _, a := range sorted[:keep] {
		out = append(out, notificationOf(a))
	}
	if rest := len(sorted) - keep; rest > 0 {
		out = append(out, Notification{
			Title:    notifyTitle,
			Body:     fmt.Sprintf("Mais %d alertas em alerts.json: veja gh board attention", rest),
			Severity: domain.SeverityWarning,
		})
	}
	return out
}

// notificationOf renders one alert the way the mod toasts it.
func notificationOf(a domain.Alert) Notification {
	body := a.Message
	if a.Item.Number > 0 {
		body = fmt.Sprintf("%s (%s#%d)", a.Message, a.Item.Repository, a.Item.Number)
	}
	// A body opening with a dash would read as an option of the notifier.
	body = strings.TrimLeft(body, "- ")
	return Notification{Title: notifyTitle, Body: body, Severity: a.Severity}
}

// notifier picks the sender of the operating system: osascript on macOS,
// a PowerShell toast on Windows and notify-send everywhere else, which
// covers Linux and the BSDs. A missing program means the text is printed.
func notifier(opts NotifyOptions) (sender, error) {
	switch opts.GOOS {
	case goosDarwin:
		return pathSender(opts, "osascript", osascriptCommand), nil
	case goosWindows:
		return windowsSender(opts)
	default:
		return pathSender(opts, "notify-send", notifySendCommand), nil
	}
}

// pathSender runs the named program when it is on the PATH and prints
// otherwise.
func pathSender(opts NotifyOptions, name string, build func(exe string, n Notification) Command) sender {
	exe, err := opts.Runner.LookPath(name)
	if err != nil {
		return printer(opts.Out)
	}
	return func(ctx context.Context, n Notification) error {
		_, err := opts.Runner.Run(ctx, build(exe, n))
		return err
	}
}

// printer writes one line per notification, severity first.
func printer(out io.Writer) sender {
	return func(_ context.Context, n Notification) error {
		_, err := fmt.Fprintf(out, "[%s] %s\n", n.Severity, n.Body)
		return err
	}
}

// osascriptCommand passes the texts as script arguments: the statements
// are fixed and read argv, so no text ever becomes AppleScript.
func osascriptCommand(exe string, n Notification) Command {
	return Command{Name: exe, Args: []string{
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		n.Title, n.Body,
	}}
}

// notifySendCommand maps the severity to the freedesktop urgency and ends
// the options before the texts.
func notifySendCommand(exe string, n Notification) Command {
	return Command{Name: exe, Args: []string{
		"--app-name=" + notifyTitle, "--urgency=" + urgency(n.Severity), "--", n.Title, n.Body,
	}}
}

// urgency is the notify-send level of a severity.
func urgency(s domain.Severity) string {
	switch s {
	case domain.SeverityCritical:
		return "critical"
	case domain.SeverityWarning:
		return "normal"
	default:
		return "low"
	}
}

// windowsSender writes the fixed toast script under Dir and runs it with
// the texts as named parameters.
func windowsSender(opts NotifyOptions) (sender, error) {
	exe, err := opts.Runner.LookPath("powershell")
	if err != nil {
		return printer(opts.Out), nil
	}
	if opts.Dir == "" {
		return nil, errors.New("notify: a directory for the toast script is required on windows")
	}
	script := filepath.Join(opts.Dir, toastScriptName)
	if err := audit.WriteAtomic(script, []byte(toastScript)); err != nil {
		return nil, fmt.Errorf("notify: %w", err)
	}
	return func(ctx context.Context, n Notification) error {
		_, err := opts.Runner.Run(ctx, powershellCommand(exe, script, n))
		return err
	}, nil
}

// powershellCommand runs the toast script by file, so that the texts bind
// to its parameters instead of being parsed as code.
func powershellCommand(exe, script string, n Notification) Command {
	return Command{Name: exe, Args: []string{
		"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass",
		"-File", script, "-Title", n.Title, "-Body", n.Body,
	}}
}
