package commands

import "github.com/spf13/cobra"

func registerWriteCommands(root *cobra.Command, deps *Deps) {
	root.AddGroup(&cobra.Group{ID: GroupWrite, Title: "Direct writes (reversible, journaled):"})
	root.AddCommand(writeNewCommand(deps), writeMoveCommand(deps), writeAssignCommand(deps),
		writeUnassignCommand(deps), writeSetCommand(deps), writeEstimateCommand(deps),
		writeDatesCommand(deps), writeCommentCommand(deps), writeLinkCommand(deps),
		writeLabelCommand(deps), writeReopenCommand(deps), writeRestoreCommand(deps))
	commandParent(root, "sprint", GroupRead).AddCommand(writeSprintCommand(deps))
	commandParent(root, "milestone", GroupPlan).AddCommand(writeMilestoneCommand(deps))
}

func writeCommand(use, short string, args cobra.PositionalArgs, run func(*cobra.Command, []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, GroupID: GroupWrite, Args: args, RunE: run}
}
