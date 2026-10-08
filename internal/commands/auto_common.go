package commands

import (
	"fmt"
	"io"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
	"github.com/cristianargotti/gh-board/internal/render"
)

// Seams over the calls whose effects reach outside the process or the
// alert state store: the notifier, the schedulers, the guard answer, the
// state diff and the state writer. Production never changes them; the
// tests of this tier replace them so that no unit test pops a notification
// or registers a job.
var (
	autoNotify             = alerts.Notify
	autoInstallScheduler   = alerts.InstallScheduler
	autoUninstallScheduler = alerts.UninstallScheduler
	autoDiffStates         = alerts.Evaluate
	autoWriteState         = alerts.WriteState
	autoGuardCheck         = guard.CheckWith
)

// autoRender prints a document in the format the global flags select.
func autoRender(deps *Deps, doc *render.Document) error {
	format := render.FormatTable
	switch {
	case deps.Flags.JSON:
		format = render.FormatJSON
	case deps.Flags.Format != "":
		parsed, err := render.ParseFormat(deps.Flags.Format)
		if err != nil {
			return err
		}
		format = parsed
	}
	return render.Render(deps.Out, doc, format)
}

// autoDiffWriter is where the installers print diffs: stdout, or stderr
// when stdout carries the JSON a script reads.
func autoDiffWriter(deps *Deps) io.Writer {
	if deps.Flags.JSON && deps.Err != nil {
		return deps.Err
	}
	return deps.Out
}

// autoWarn prints a non-fatal problem on stderr.
func autoWarn(deps *Deps, format string, args ...any) {
	if deps.Err == nil {
		return
	}
	_, _ = fmt.Fprintf(deps.Err, format+"\n", args...)
}

// autoLoadConfig resolves board.yml with the global flags and keeps the
// result in deps for the rest of the command.
func autoLoadConfig(deps *Deps) (config.Loaded, error) {
	var ref domain.ProjectRef
	if deps.Flags.Project != "" {
		parsed, err := domain.ParseProjectRef(deps.Flags.Project)
		if err != nil {
			return config.Loaded{}, err
		}
		ref = parsed
	}
	loaded, err := config.Load(config.LoadOptions{Path: deps.Flags.Config, Dirs: deps.Dirs, Project: ref})
	if err != nil {
		return config.Loaded{}, err
	}
	deps.Config = &loaded
	return loaded, nil
}

// autoRequireProject returns the configured project or a usage error.
func autoRequireProject(loaded config.Loaded) (domain.ProjectRef, error) {
	if loaded.Config == nil || loaded.Config.Project.IsZero() {
		return domain.ProjectRef{}, readNoProject(&loaded)
	}
	return loaded.Config.Project, nil
}
