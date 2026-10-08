package commands

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeReopenCommand(deps *Deps) *cobra.Command {
	return writeCommand("reopen <ref[,ref...]>", "Reopen issues", cobra.ExactArgs(1),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error {
					if it.Issue.State == domain.IssueOpen {
						return nil
					}
					s.writeAdd(it, domain.OpReopenIssue, "state", string(domain.IssueOpen), "Reabrir issue.",
						func(ctx context.Context, current domain.Item) error {
							if current.Issue.State == domain.IssueOpen {
								return nil
							}
							return s.deps.Writer.ReopenIssue(ctx, current.Issue.NodeID)
						})
					return nil
				})
			})
		})
}
