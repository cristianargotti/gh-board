package commands

import "github.com/spf13/cobra"

// Tiers share parents so registering a later tier cannot replace earlier commands.
func commandParent(root *cobra.Command, name, group string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	cmd := &cobra.Command{Use: name, Short: "Inspect or manage " + name, GroupID: group, Args: cobra.NoArgs}
	cmd.AddGroup(
		&cobra.Group{ID: GroupRead, Title: "Read:"},
		&cobra.Group{ID: GroupWrite, Title: "Direct writes:"},
		&cobra.Group{ID: GroupPlan, Title: "Plans:"},
	)
	root.AddCommand(cmd)
	return cmd
}
