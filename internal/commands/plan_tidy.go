package commands

import (
	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func planTidyCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use: "tidy", Short: "Plan the routines declared in board.yml", GroupID: GroupPlan, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pc, err := planStart(cmd.Context(), deps)
			if err != nil {
				return err
			}
			items, err := pc.planItems(cmd.Context())
			if err != nil {
				return err
			}
			result := domain.BuildTidyPlan(domain.TidyInput{Config: pc.cfg, Project: pc.project, Items: items, Now: deps.Now()})
			if err := pc.planCheckItems(cmd.Context(), planTidyTargets(items, result.Steps)); err != nil {
				return err
			}
			if err := planTidySkipped(deps, result.Skipped); err != nil {
				return err
			}
			return pc.planSave("tidy", "Executar as rotinas declaradas no board.yml, preservando alterações concorrentes.", result.Steps)
		},
	}
}

func planTidyTargets(items []domain.Item, steps []domain.Step) []domain.Item {
	ids := make(map[string]bool)
	for _, step := range steps {
		ids[step.Target.NodeID] = true
	}
	var selected []domain.Item
	for _, item := range items {
		if ids[item.Issue.NodeID] {
			selected = append(selected, item)
		}
	}
	return selected
}

func planTidySkipped(deps *Deps, skipped []domain.TidySkip) error {
	if len(skipped) == 0 {
		return nil
	}
	doc := render.NewDocument("Skipped routines")
	section := doc.AddSection("")
	for _, skip := range skipped {
		section.AddNote(render.Title(skip.Item.Title) + ": " + render.Excerpt(render.LabelBody, skip.Reason))
	}
	// Diagnostics use stderr so JSON on stdout stays a single document; the
	// text formats keep the one the person asked for.
	snapshot := *deps
	snapshot.Out, snapshot.Flags.JSON = deps.Err, false
	if deps.Flags.JSON || snapshot.Flags.Format == "" {
		snapshot.Flags.Format = string(render.FormatCompact)
	}
	return planRender(&snapshot, doc)
}
