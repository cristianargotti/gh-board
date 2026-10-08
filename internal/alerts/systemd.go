package alerts

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
)

// systemdUserDir holds the per-user units.
const systemdUserDir = ".config/systemd/user"

// unitPath is one unit file under the home.
func (j *job) unitPath(ext string) string {
	return filepath.Join(j.opts.Home, systemdUserDir, jobName+ext)
}

// systemctl builds a user-instance systemctl command.
func systemctl(args ...string) Command {
	return Command{Name: "systemctl", Args: append([]string{"--user"}, args...)}
}

// installLinux prefers the systemd user timer and falls back to cron when
// systemctl is absent.
func (j *job) installLinux(ctx context.Context) error {
	if _, err := j.opts.Runner.LookPath("systemctl"); err != nil {
		j.say("systemd is absent: installing a cron line\n")
		return j.installCron(ctx)
	}
	return j.installSystemd(ctx)
}

// installSystemd writes the service and the timer, then enables the timer.
func (j *job) installSystemd(ctx context.Context) error {
	if err := j.write(j.unitPath(".service"), j.serviceUnit()); err != nil {
		return err
	}
	if err := j.write(j.unitPath(".timer"), j.timerUnit()); err != nil {
		return err
	}
	if _, err := j.run(ctx, systemctl("daemon-reload")); err != nil {
		return err
	}
	_, err := j.run(ctx, systemctl("enable", "--now", jobName+".timer"))
	return err
}

// uninstallLinux removes the units when systemd is there and the cron line
// when cron is there, so that whichever install wrote is gone.
func (j *job) uninstallLinux(ctx context.Context) error {
	removed := false
	if _, err := j.opts.Runner.LookPath("systemctl"); err == nil {
		done, err := j.uninstallSystemd(ctx)
		if err != nil {
			return err
		}
		removed = removed || done
	}
	if _, err := j.opts.Runner.LookPath("crontab"); err == nil {
		done, err := j.uninstallCron(ctx)
		if err != nil {
			return err
		}
		removed = removed || done
	}
	if !removed {
		j.say("nothing to remove\n")
	}
	return nil
}

// uninstallSystemd stops the timer and removes both units. A timer that
// is not active makes the disable fail, which is not an error here.
func (j *job) uninstallSystemd(ctx context.Context) (bool, error) {
	service, timer := j.unitPath(".service"), j.unitPath(".timer")
	if !exists(service) && !exists(timer) {
		return false, nil
	}
	_, _ = j.run(ctx, systemctl("disable", "--now", jobName+".timer"))
	for _, path := range []string{timer, service} {
		if err := j.remove(path); err != nil {
			return false, err
		}
	}
	_, err := j.run(ctx, systemctl("daemon-reload"))
	return true, err
}

// serviceUnit renders the oneshot service that runs watch.
func (j *job) serviceUnit() string {
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=gh board watch of " + j.opts.Project.String() + "\n\n")
	b.WriteString("[Service]\nType=oneshot\n")
	if j.ghPath != "" {
		b.WriteString("Environment=" + systemdQuote(envGHPath+"="+j.ghPath) + "\n")
	}
	args := make([]string, 0, len(j.command()))
	for _, arg := range j.command() {
		args = append(args, systemdQuote(arg))
	}
	b.WriteString("ExecStart=" + strings.Join(args, " ") + "\n")
	return b.String()
}

// timerUnit renders the timer: a first run a minute after login, then one
// every interval.
func (j *job) timerUnit() string {
	every := strconv.Itoa(minutes(j.opts.Interval)) + "min"
	return "[Unit]\nDescription=gh board watch every " + every + "\n\n" +
		"[Timer]\nOnStartupSec=1min\nOnUnitActiveSec=" + every + "\nUnit=" + jobName + ".service\n\n" +
		"[Install]\nWantedBy=timers.target\n"
}

// systemdQuote wraps a value in the double quotes unit files understand.
func systemdQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
