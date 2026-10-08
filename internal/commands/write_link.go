package commands

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeLinkCommand(deps *Deps) *cobra.Command {
	var parent string
	cmd := writeCommand("link <child[,child...]>", "Link work under a parent issue", cobra.ExactArgs(1),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				if parent == "" {
					return domain.Errorf(domain.ExitUsage, "provide --parent")
				}
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeLink(it, parent) })
			})
		})
	cmd.Flags().StringVar(&parent, "parent", "", "parent issue reference")
	return cmd
}

func (s *writeSession) writeLink(it domain.Item, raw string) error {
	parent, err := s.writeResolve(raw)
	if err != nil {
		return err
	}
	if parent.Issue.NodeID == it.Issue.NodeID {
		return domain.Errorf(domain.ExitUsage, "an issue cannot be its own parent")
	}
	if it.Issue.Parent != nil && it.Issue.Parent.NodeID != parent.Issue.NodeID {
		return domain.Errorf(domain.ExitPolicy, "issue already has a different parent; unlinking is unavailable")
	}
	// The parent travels as owner/repo#n, the form the precondition reads
	// and apply resolves, so the plan and the output name it for a reader.
	s.writeAdd(it, domain.OpAddSubIssue, "parent", parent.Issue.Ref(), "Vincular ao item pai.",
		func(ctx context.Context, current domain.Item) error {
			if _, err := s.writeFresh(writeTarget(parent)); err != nil {
				return err
			}
			child, err := s.writeFresh(writeTarget(current))
			if err != nil {
				return err
			}
			if child.Issue.Parent != nil && child.Issue.Parent.NodeID != parent.Issue.NodeID {
				return domain.Errorf(domain.ExitDrift, "parent changed before link")
			}
			if child.Issue.Parent != nil {
				return nil
			}
			return s.deps.Writer.AddSubIssue(ctx, parent.Issue.NodeID, child.Issue.NodeID)
		})
	return nil
}
