package alerts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// jobName names the watch job in every scheduler: the launchd label and
// plist, the systemd units, the cron marker and the Task Scheduler task.
const jobName = "gh-board.watch"

// envGHPath is the variable go-gh reads to find gh for the keyring token.
// Schedulers start jobs with a minimal PATH, so the job carries the path
// gh had at install time.
const envGHPath = "GH_PATH"

// SchedulerOptions describe the watch job to register as the user.
type SchedulerOptions struct {
	GOOS     string
	Binary   string
	Project  domain.ProjectRef
	Interval time.Duration
	DryRun   bool
	Out      io.Writer
	// Home is the user's home directory, where launchd and systemd keep
	// the agent files. The command resolves it: this package reads no
	// environment.
	Home string
	// UID is the user id of the launchd domain gui/<uid>; zero means the
	// current user.
	UID int
	// Runner launches launchctl, systemctl, crontab and schtasks; nil
	// means ExecRunner.
	Runner Runner
}

// job is a scheduler operation in progress: the completed options and the
// executable paths install resolved. headless is the Windows console host
// that runs the task without a visible window.
type job struct {
	opts     SchedulerOptions
	binary   string
	ghPath   string
	headless string
}

// newJob fills the zero options and checks the ones every scheduler needs.
func newJob(opts SchedulerOptions) (*job, error) {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Runner == nil {
		opts.Runner = ExecRunner{}
	}
	if opts.Interval == 0 {
		opts.Interval = DefaultInterval
	}
	if opts.Home == "" && opts.GOOS != goosWindows {
		return nil, fmt.Errorf("scheduler: the home directory is unknown: %w", domain.ErrUsage)
	}
	return &job{opts: opts}, nil
}

// prepare resolves what install writes: the project the job polls, the
// interval, the executable by its absolute path and the gh path.
func (j *job) prepare() error {
	if j.opts.Project.IsZero() {
		return fmt.Errorf("scheduler: a project is required: %w", domain.ErrUsage)
	}
	if err := validateInterval(j.opts.Interval); err != nil {
		return fmt.Errorf("scheduler: %w", err)
	}
	binary, err := resolveBinary(j.opts.Binary)
	if err != nil {
		return fmt.Errorf("scheduler: %w", err)
	}
	j.binary = binary
	if gh, err := j.opts.Runner.LookPath("gh"); err == nil {
		j.ghPath = gh
	}
	if j.opts.GOOS == goosWindows {
		if host, err := j.opts.Runner.LookPath(headlessHost); err == nil {
			j.headless = host
		}
	}
	return nil
}

// resolveBinary returns the absolute path of the executable the job runs,
// symlinks followed; empty means the running binary.
func resolveBinary(binary string) (string, error) {
	var err error
	if binary == "" {
		if binary, err = os.Executable(); err != nil {
			return "", err
		}
	}
	if binary, err = filepath.Abs(binary); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return "", fmt.Errorf("executable %s: %w", binary, err)
	}
	return resolved, nil
}

// command is the watch invocation the scheduler runs.
func (j *job) command() []string {
	return []string{j.binary, "watch", "--once", "--project", j.opts.Project.String()}
}

// say reports one step on the output.
func (j *job) say(format string, args ...any) {
	_, _ = fmt.Fprintf(j.opts.Out, format, args...)
}

// write puts a file in place, or prints it in a dry run.
func (j *job) write(path, content string) error {
	if j.opts.DryRun {
		j.say("would write %s:\n%s", path, content)
		return nil
	}
	if err := audit.WriteAtomic(path, []byte(content)); err != nil {
		return fmt.Errorf("scheduler: %w", err)
	}
	j.say("wrote %s\n", path)
	return nil
}

// remove deletes a file install wrote; a file already gone is fine.
func (j *job) remove(path string) error {
	if j.opts.DryRun {
		j.say("would remove %s\n", path)
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("scheduler: %w", err)
	}
	j.say("removed %s\n", path)
	return nil
}

// run launches a scheduler command, or prints it in a dry run, input
// included so that the dry run shows exactly what would be written.
func (j *job) run(ctx context.Context, cmd Command) ([]byte, error) {
	if j.opts.DryRun {
		j.say("would run %s\n", cmd)
		if len(cmd.Stdin) > 0 {
			j.say("with input:\n%s", cmd.Stdin)
		}
		return nil, nil
	}
	out, err := j.opts.Runner.Run(ctx, cmd)
	if err != nil {
		return out, fmt.Errorf("scheduler: %w", err)
	}
	j.say("ran %s\n", cmd)
	return out, nil
}

// exists reports whether a file install may have written is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// InstallScheduler registers watch --once with the native scheduler: a
// launchd agent, a systemd user timer (cron when systemd is absent) or a
// Task Scheduler task. DryRun prints what would be written and run.
func InstallScheduler(ctx context.Context, opts SchedulerOptions) error {
	j, err := newJob(opts)
	if err != nil {
		return err
	}
	if err := j.prepare(); err != nil {
		return err
	}
	switch j.opts.GOOS {
	case goosDarwin:
		return j.installLaunchd(ctx)
	case goosLinux:
		return j.installLinux(ctx)
	case goosWindows:
		return j.installSchtasks(ctx)
	default:
		return fmt.Errorf("scheduler: no scheduler for %s: %w", j.opts.GOOS, domain.ErrUsage)
	}
}

// UninstallScheduler removes exactly what InstallScheduler registered.
func UninstallScheduler(ctx context.Context, opts SchedulerOptions) error {
	j, err := newJob(opts)
	if err != nil {
		return err
	}
	switch j.opts.GOOS {
	case goosDarwin:
		return j.uninstallLaunchd(ctx)
	case goosLinux:
		return j.uninstallLinux(ctx)
	case goosWindows:
		return j.uninstallSchtasks(ctx)
	default:
		return fmt.Errorf("scheduler: no scheduler for %s: %w", j.opts.GOOS, domain.ErrUsage)
	}
}
