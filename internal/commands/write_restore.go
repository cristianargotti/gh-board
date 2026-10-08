package commands

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeRestoreCommand(deps *Deps) *cobra.Command {
	return writeCommand("restore <ref[,ref...]>", "Restore archived project items", cobra.ExactArgs(1),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error {
					if !it.Archived {
						return nil
					}
					s.writeAdd(it, domain.OpUnarchiveItem, "archived", domain.ActiveValue, "Restaurar item no projeto.",
						func(ctx context.Context, current domain.Item) error {
							if !current.Archived {
								return nil
							}
							if current.ProjectItemID == "" {
								return domain.Errorf(domain.ExitNotFound, "target is not in the project")
							}
							return s.deps.Writer.UnarchiveItem(ctx, s.project.NodeID, current.ProjectItemID)
						})
					return nil
				})
			})
		})
}
