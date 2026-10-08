package alerts

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// cronMarker tags the crontab line watch owns. It is a shell comment at
// the end of the line: cron hands the command to sh, which drops it.
const cronMarker = "# " + jobName

// shellSafe matches the arguments a POSIX shell takes without quotes.
var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./:@+=,-]+$`)

// crontabCommand builds a crontab command.
func crontabCommand(stdin []byte, args ...string) Command {
	return Command{Name: "crontab", Args: args, Stdin: stdin}
}

// installCron replaces the marked line of the crontab, keeping every other
// line as it is.
func (j *job) installCron(ctx context.Context) error {
	lines, err := j.crontab(ctx)
	if err != nil {
		return err
	}
	line, err := j.cronLine()
	if err != nil {
		return err
	}
	lines = append(withoutMarker(lines), line)
	_, err = j.run(ctx, crontabCommand([]byte(strings.Join(lines, "\n")+"\n"), "-"))
	return err
}

// uninstallCron drops the marked line; the crontab goes away with it when
// nothing else is left, as before the install.
func (j *job) uninstallCron(ctx context.Context) (bool, error) {
	lines, err := j.crontab(ctx)
	if err != nil {
		return false, err
	}
	kept := withoutMarker(lines)
	if len(kept) == len(lines) {
		return false, nil
	}
	if len(kept) == 0 {
		_, err = j.run(ctx, crontabCommand(nil, "-r"))
		return true, err
	}
	_, err = j.run(ctx, crontabCommand([]byte(strings.Join(kept, "\n")+"\n"), "-"))
	return true, err
}

// crontab reads the user's crontab, always, because a dry run must show
// the result; a user without one has an empty crontab.
func (j *job) crontab(ctx context.Context) ([]string, error) {
	out, err := j.opts.Runner.Run(ctx, crontabCommand(nil, "-l"))
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "no crontab") {
			return nil, nil
		}
		return nil, fmt.Errorf("scheduler: %w", err)
	}
	text := strings.TrimRight(string(out), "\r\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// withoutMarker keeps the lines watch does not own.
func withoutMarker(lines []string) []string {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasSuffix(strings.TrimSpace(line), cronMarker) {
			kept = append(kept, line)
		}
	}
	return kept
}

// cronLine renders the job: schedule, the gh path for the keyring token,
// the quoted command and the marker. A percent sign would end the command
// for cron, so it is escaped.
func (j *job) cronLine() (string, error) {
	schedule, err := cronSchedule(j.opts.Interval)
	if err != nil {
		return "", err
	}
	parts := []string{schedule}
	if j.ghPath != "" {
		parts = append(parts, envGHPath+"="+shellQuote(j.ghPath))
	}
	for _, arg := range j.command() {
		parts = append(parts, shellQuote(arg))
	}
	parts = append(parts, cronMarker)
	return strings.ReplaceAll(strings.Join(parts, " "), "%", `\%`), nil
}

// cronSchedule expresses the interval in the five cron fields: minutes
// under an hour, whole hours, or once a day.
func cronSchedule(d time.Duration) (string, error) {
	m := minutes(d)
	switch {
	case m < 60:
		return fmt.Sprintf("*/%d * * * *", m), nil
	case m == 24*60:
		return "0 0 * * *", nil
	case m%60 == 0:
		return fmt.Sprintf("0 */%d * * *", m/60), nil
	}
	return "", fmt.Errorf("scheduler: interval %s: cron takes up to 59 minutes or whole hours: %w", d, domain.ErrUsage)
}

// shellQuote single-quotes an argument unless the shell takes it as is.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
