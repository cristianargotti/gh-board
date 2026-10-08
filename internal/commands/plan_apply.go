package commands

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/cristianargotti/gh-board/internal/render"
)

func planApplyCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use: "apply <id|path>", Short: "Apply a reviewed plan from an interactive terminal", GroupID: GroupPlan, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := plan.Read(deps.Dirs.State, args[0])
			if err != nil {
				return err
			}
			viewer, err := planViewer(cmd.Context(), deps)
			if err != nil {
				return err
			}
			input, _ := cmd.InOrStdin().(*os.File)
			return planApplyRun(cmd.Context(), deps, p, viewer, plan.IsTerminal(input))
		},
	}
}

func planApplyRun(ctx context.Context, deps *Deps, p domain.Plan, viewer domain.Viewer, interactive bool) error {
	if viewer.Host != p.Host {
		return domain.Errorf(domain.ExitApplyRefused, "plan host differs from the current viewer host")
	}
	if _, err := planFormat(deps); err != nil {
		return err
	}
	writer := &planApplyWriter{ProjectWriter: deps.Writer, reader: deps.Reader, state: deps.Dirs.State, p: p}
	if err := writer.planApplyPreflight(ctx); err != nil {
		return err
	}
	result, err := plan.Apply(ctx, plan.Ports{Reader: deps.Reader, Writer: writer, Clock: deps.Clock}, p,
		plan.ApplyOptions{DryRun: deps.Flags.DryRun, Interactive: interactive, Viewer: viewer, StateDir: deps.Dirs.State, Out: deps.Err})
	if err != nil {
		return err
	}
	if !deps.Flags.DryRun {
		if err := planApplyLocal(ctx, deps, writer, viewer); err != nil {
			return err
		}
	}
	doc := render.NewDocument("Apply " + p.ID)
	doc.Data = result
	section := doc.AddSection("")
	section.AddNote(result.String())
	for _, step := range p.Steps {
		section.AddNote(render.Sanitize(step.Description, 0))
	}
	return planRender(deps, doc)
}

func planApplyLocal(ctx context.Context, deps *Deps, writer *planApplyWriter, viewer domain.Viewer) error {
	if writer.p.Command != planCommandInit {
		return nil
	}
	err := writer.planFinishInit(ctx)
	entry := audit.Entry{
		At: deps.Now(), Actor: viewer.Login, Host: viewer.Host, Project: writer.p.Project,
		Command: "init local", PlanID: writer.p.ID, Reason: deps.Flags.Reason, Result: "ok",
	}
	if err != nil {
		entry.Result, entry.Error = "failed", err.Error()
	}
	auditErr := audit.Append(deps.Dirs.State, entry)
	if err != nil {
		return err
	}
	return auditErr
}
