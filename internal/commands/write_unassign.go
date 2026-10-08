package commands

import (
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeUnassignCommand(deps *Deps) *cobra.Command {
	cmd := writeCommand("unassign <ref[,ref...]> <login>", "Remove an assignee", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeAssignee(it, args[1], false) })
			})
		})
	cmd.Annotations = map[string]string{mcpArgsAnnotation: "ref: Issue references separated by commas: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number\n" +
		"login: GitHub login of the person to remove, with or without the leading @"}
	return cmd
}
