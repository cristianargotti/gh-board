package commands

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// autoUsePayload is the --json form of use.
type autoUsePayload struct {
	Project  string `json:"project,omitempty"`
	Title    string `json:"title,omitempty"`
	Path     string `json:"path"`
	Recorded bool   `json:"recorded"`
	Cleared  bool   `json:"cleared"`
}

// autoUseCommand builds "use [owner/number] [--clear]": the default
// project of section 5.3, so that no command needs --project.
func autoUseCommand(deps *Deps) *cobra.Command {
	var forget bool
	cmd := &cobra.Command{
		Use:     "use [owner/number]",
		Short:   "Record the default project every command reads when nothing else selects one",
		GroupID: GroupAuto,
		Args:    cobra.MaximumNArgs(1),
		Long: "use records owner/number as the default project in the per-user configuration directory. " +
			"Every command reads it when no --project, GH_BOARD_CONFIG or board.yml selects a project. " +
			"Without an argument it prints the current default; --clear forgets it.",
		RunE: func(cmd *cobra.Command, args []string) error { return autoUse(cmd.Context(), deps, args, forget) },
	}
	cmd.Flags().BoolVar(&forget, "clear", false, "forget the default project")
	return cmd
}

func autoUse(ctx context.Context, deps *Deps, args []string, forget bool) error {
	path := config.DefaultPath(deps.Dirs.Config)
	if path == "" {
		return domain.Errorf(domain.ExitUsage, "use needs the configuration directory")
	}
	switch {
	case forget && len(args) > 0:
		return domain.Errorf(domain.ExitUsage, "--clear takes no project")
	case forget:
		return autoUseClear(deps, path)
	case len(args) == 0:
		return autoUseShow(deps)
	}
	return autoUseRecord(ctx, deps, path, args[0])
}

// autoUseRecord confirms the project through discovery before it writes,
// so a typo never becomes the default of every command.
func autoUseRecord(ctx context.Context, deps *Deps, path, raw string) error {
	ref, err := domain.ParseProjectRef(raw)
	if err != nil {
		return err
	}
	if deps.Reader == nil {
		return domain.Errorf(domain.ExitUsage, "a GitHub reader is required")
	}
	project, err := deps.Reader.DiscoverProject(ctx, ref)
	if err != nil {
		return readAPI(err)
	}
	if err := audit.WriteAtomic(path, config.EncodeDefault(ref)); err != nil {
		return err
	}
	payload := autoUsePayload{Project: ref.String(), Title: render.Title(project.Title), Path: path, Recorded: true}
	doc := render.NewDocument("Default project")
	sec := doc.AddSection("")
	sec.AddKeyValue(readKeyProject, payload.Project).AddKeyValue(readKeyTitle, payload.Title).AddKeyValue("File", path)
	sec.AddNote("every command reads it when no --project, GH_BOARD_CONFIG or board.yml selects a project")
	doc.Data = payload
	return autoRender(deps, doc)
}

// autoUseShow prints the recorded default, or how to record one.
func autoUseShow(deps *Deps) error {
	def, err := config.ReadDefault(deps.Dirs.Config)
	if err != nil {
		return err
	}
	payload := autoUsePayload{Path: config.DefaultPath(deps.Dirs.Config)}
	doc := render.NewDocument("Default project")
	sec := doc.AddSection("")
	if def.IsZero() {
		sec.AddNote("no default project: run gh board use owner/number")
	} else {
		payload.Project, payload.Recorded = def.Project.String(), true
		sec.AddKeyValue(readKeyProject, payload.Project).AddKeyValue("File", def.Path)
	}
	doc.Data = payload
	return autoRender(deps, doc)
}

// autoUseClear removes the recorded default; none recorded is not an
// error, the outcome is the same.
func autoUseClear(deps *Deps, path string) error {
	payload := autoUsePayload{Path: path}
	doc := render.NewDocument("Default project")
	sec := doc.AddSection("")
	switch err := os.Remove(path); {
	case errors.Is(err, fs.ErrNotExist):
		sec.AddNote("no default project recorded")
	case err != nil:
		return err
	default:
		payload.Cleared = true
		sec.AddNote("default project cleared: " + path)
	}
	doc.Data = payload
	return autoRender(deps, doc)
}
