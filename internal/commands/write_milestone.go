package commands

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeMilestoneCommand(deps *Deps) *cobra.Command {
	return writeCommand("set <ref[,ref...]> <title>", "Set a repository milestone", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeMilestone(it, args[1]) })
			})
		})
}

func (s *writeSession) writeMilestone(it domain.Item, title string) error {
	milestones, err := s.deps.Reader.RepositoryMilestones(s.ctx, it.Issue.Owner, it.Issue.Repo)
	if err != nil {
		return writeAPI(err)
	}
	milestone, err := planResolveMilestone(milestones, title)
	if err != nil {
		return err
	}
	s.writeAdd(it, domain.OpUpdateIssue, domain.UpdateFieldMilestone, milestone.Title, "Definir marco de entrega.",
		func(ctx context.Context, current domain.Item) error {
			_, err := s.deps.Writer.UpdateIssue(ctx, domain.UpdateIssueInput{IssueID: current.Issue.NodeID, MilestoneID: &milestone.ID})
			return err
		})
	return nil
}
