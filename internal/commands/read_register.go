package commands

import "github.com/spf13/cobra"

// registerReadCommands registers the read tier of section 6.3: context,
// status, me, item, list, epics, roadmap, sprint current|next, attention,
// search, schema, digest print, standup, doctor, log and version. The
// plan tier registers plan list|show under the shared "plan" parent.
func registerReadCommands(root *cobra.Command, deps *Deps) {
	root.AddGroup(&cobra.Group{ID: GroupRead, Title: "Read commands (no side effects):"})
	root.AddCommand(
		readContextCommand(deps), readStatusCommand(deps), readMeCommand(deps), readItemCommand(deps),
		readListCommand(deps), readEpicsCommand(deps), readRoadmapCommand(deps), readAttentionCommand(deps),
		readSearchCommand(deps), readSchemaCommand(deps), readDoctorCommand(deps), readLogCommand(deps),
		readVersionCommand(deps), readStandupCommand(deps),
	)
	sprint := commandParent(root, "sprint", GroupRead)
	sprint.Short = "Inspect the current or next sprint, or set an item's sprint"
	current := readSprintCurrentCommand(deps)
	sprint.RunE = current.RunE
	sprint.AddCommand(current, readSprintNextCommand(deps))
	digest := commandParent(root, "digest", GroupRead)
	digest.Short = "Print the weekly digest or plan a post"
	printCommand := readDigestPrintCommand(deps)
	digest.RunE = printCommand.RunE
	digest.Flags().AddFlagSet(printCommand.Flags())
	digest.AddCommand(printCommand)
}
