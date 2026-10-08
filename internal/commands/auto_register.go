package commands

import "github.com/spf13/cobra"

// registerAutoCommands registers the automation tier of section 7: use,
// watch, agent install|uninstall, guard check, guard install|uninstall and
// the MCP server of section 8.4.
func registerAutoCommands(root *cobra.Command, deps *Deps) {
	root.AddGroup(&cobra.Group{ID: GroupAuto, Title: "Automation and agents:"})
	root.AddCommand(autoUseCommand(deps), autoWatchCommand(deps), autoAgentCommand(deps), autoGuardCommand(deps), autoMCPCommand(deps))
}
