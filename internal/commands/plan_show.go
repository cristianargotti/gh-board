package commands

import (
	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/plan"
)

func planShowCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use: "show <id|path>", Short: "Inspect a saved plan", GroupID: GroupRead, Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := plan.Read(deps.Dirs.State, args[0])
			if err != nil {
				return err
			}
			return planPresent(deps, p, "")
		},
	}
}
