package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoWatchOptions are the flags of watch.
type autoWatchOptions struct {
	Once      bool
	Install   bool
	Uninstall bool
	Interval  string
}

// autoWatchCommand builds "watch [--once] [--install [--interval]]
// [--uninstall]" (section 9).
func autoWatchCommand(deps *Deps) *cobra.Command {
	var opts autoWatchOptions
	cmd := &cobra.Command{
		Use:     "watch",
		Short:   "Evaluate the alerts once, or register the native scheduler that does it",
		GroupID: GroupAuto,
		Args:    cobra.NoArgs,
		Long: "watch evaluates the alert rules over the board, diffs them against the last run, notifies only the new " +
			"and escalated alerts through the native notifier and writes alerts.json for the Claude Code mod. Without " +
			"--install or --uninstall it runs once, which is also what --once says. --install registers watch --once " +
			"as the user with launchd, a systemd user timer (cron when systemd is absent) or the Task Scheduler; one " +
			"poller only. --dry-run shows what a run would notify and write.",
		RunE: func(c *cobra.Command, _ []string) error { return autoRunWatch(c.Context(), deps, opts) },
	}
	f := cmd.Flags()
	f.BoolVar(&opts.Once, "once", false, "evaluate, notify and write alerts.json once (the default)")
	f.BoolVar(&opts.Install, "install", false, "register watch --once with the native scheduler")
	f.BoolVar(&opts.Uninstall, "uninstall", false, "remove the scheduler entry that --install registered")
	f.StringVar(&opts.Interval, "interval", "", "interval of the scheduler for --install, a duration such as 10m or a number of minutes (default 10m)")
	cmd.MarkFlagsMutuallyExclusive("once", "install", "uninstall")
	return cmd
}

// autoRunWatch dispatches on the mode flags.
func autoRunWatch(ctx context.Context, deps *Deps, opts autoWatchOptions) error {
	loaded, err := autoLoadConfig(deps)
	if err != nil {
		return err
	}
	switch {
	case opts.Install:
		return autoWatchScheduler(ctx, deps, loaded, opts.Interval, true)
	case opts.Uninstall:
		return autoWatchScheduler(ctx, deps, loaded, opts.Interval, false)
	}
	return autoWatchOnce(ctx, deps, loaded)
}

// autoWatchScheduler registers or removes the native scheduler entry. The
// binary is named by its resolved path (section 11) and the home comes
// from the command, because the alerts package reads no environment.
func autoWatchScheduler(ctx context.Context, deps *Deps, loaded config.Loaded, interval string, install bool) error {
	project, err := autoRequireProject(loaded)
	if err != nil {
		return err
	}
	every, err := alerts.ParseInterval(interval)
	if err != nil {
		return err
	}
	binary, err := guard.Binary()
	if err != nil {
		return err
	}
	home, err := autoUserHome()
	if err != nil {
		return fmt.Errorf("resolve the home directory: %w", err)
	}
	sopts := alerts.SchedulerOptions{
		Binary:   binary,
		Project:  project,
		Interval: every,
		DryRun:   deps.Flags.DryRun,
		Out:      deps.Out,
		Home:     home,
	}
	if install {
		return autoInstallScheduler(ctx, sopts)
	}
	return autoUninstallScheduler(ctx, sopts)
}
