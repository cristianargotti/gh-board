package alerts

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// headlessHost is the Windows console host. A task that runs only when
// the user is logged on (the one form that needs no stored password and
// keeps the keyring and the network) opens a console window for a console
// program; conhost --headless runs the program on a pseudo console
// instead, so nothing flashes on the desktop every interval.
const headlessHost = "conhost.exe"

// schtasks builds a Task Scheduler command.
func schtasks(args ...string) Command {
	return Command{Name: "schtasks", Args: args}
}

// installSchtasks creates or replaces the task as the user; /F replaces
// an existing task so that a second install is idempotent. No /RU or /RP
// is passed on purpose: the task runs in the interactive session of the
// person, where gh reaches the keyring and the toast reaches the desktop.
func (j *job) installSchtasks(ctx context.Context) error {
	schedule, modifier := schtasksSchedule(j.opts.Interval)
	_, err := j.run(ctx, schtasks("/Create", "/F", "/SC", schedule, "/MO", modifier, "/TN", jobName, "/TR", j.taskRun()))
	return err
}

// uninstallSchtasks deletes the task when it exists.
func (j *job) uninstallSchtasks(ctx context.Context) error {
	if _, err := j.opts.Runner.Run(ctx, schtasks("/Query", "/TN", jobName)); err != nil {
		j.say("nothing to remove: task %s is absent\n", jobName)
		return nil
	}
	_, err := j.run(ctx, schtasks("/Delete", "/F", "/TN", jobName))
	return err
}

// taskRun is the /TR value: the console host when present, then the
// executable in quotes, for the spaces of Program Files, then the
// arguments.
func (j *job) taskRun() string {
	args := j.command()
	line := strings.Join(append([]string{`"` + args[0] + `"`}, args[1:]...), " ")
	if j.headless != "" {
		return `"` + j.headless + `" --headless ` + line
	}
	return line
}

// schtasksSchedule expresses the interval as /SC and /MO: minutes, whole
// hours, or once a day.
func schtasksSchedule(d time.Duration) (string, string) {
	m := minutes(d)
	switch {
	case m == 24*60:
		return "DAILY", "1"
	case m%60 == 0:
		return "HOURLY", strconv.Itoa(m / 60)
	}
	return "MINUTE", strconv.Itoa(m)
}
