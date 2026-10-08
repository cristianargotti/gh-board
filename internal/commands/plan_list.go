package commands

import (
	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/cristianargotti/gh-board/internal/render"
)

func planListCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List saved plans", GroupID: GroupRead, Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			plans, err := plan.List(deps.Dirs.State)
			if err != nil {
				return err
			}
			doc := render.NewDocument("Plans")
			safe := make([]domain.Plan, 0, len(plans))
			table := doc.AddSection("").SetTable("ID", "Command", "Actor", "Expires", "Description")
			for _, p := range plans {
				p = planDisplay(p)
				safe = append(safe, p)
				table.AddRow(p.ID, p.Command, p.Actor, p.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"), p.Description)
			}
			doc.Data = safe
			return planRender(deps, doc)
		},
	}
}
