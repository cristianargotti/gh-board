package commands

import "github.com/spf13/cobra"

// autoAgentCommand builds "agent install|uninstall", the installers of the
// kit in Claude Code, Cursor and Codex (sections 6.5, 8.2 and 8.3).
func autoAgentCommand(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "agent",
		Short:   "Install or remove the kit in Claude Code, Cursor and Codex",
		GroupID: GroupAuto,
		Args:    cobra.NoArgs,
		RunE:    func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddGroup(&cobra.Group{ID: GroupAuto, Title: "Automation:"})
	cmd.AddCommand(
		autoSetupCommand(deps, autoSetupSpec{
			Use:   "install",
			Short: "Install the skill, the guards and the plugin pointer for an agent",
			Long: "agent install writes what each agent needs and nothing else. Claude Code: the deny rules merged into " +
				"permissions.deny of the settings file, the PreToolUse hook merged into its hooks and the plugin pointer " +
				"(marketplace and enabled plugin). Cursor: the " +
				"skill, the beforeShellExecution hook merged into hooks.json and, with --scope project, the rule file. Codex: " +
				"the skill, the PreToolUse hook merged into hooks.json, the rules file and, with --scope project, a marked " +
				"block in AGENTS.md. With --mcp, the MCP server entry (gh board mcp) of each agent as well. Every write " +
				"is idempotent, shows its diff first and is recorded so that uninstall removes exactly what was added.",
			Kinds:   autoAllKinds,
			Install: true,
			MCP:     true,
		}),
		autoSetupCommand(deps, autoSetupSpec{
			Use:   "uninstall",
			Short: "Remove everything agent install added for an agent",
			Long: "agent uninstall reverts the recorded install: files the kit owns go away, merged entries leave the " +
				"agent's own settings untouched and the marked block leaves the rest of AGENTS.md as it was.",
			Kinds: autoAllKinds,
			MCP:   true,
		}),
	)
	return cmd
}
