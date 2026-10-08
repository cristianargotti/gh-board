package commands

import "github.com/spf13/cobra"

const planCommandInit = "init"

func registerPlanCommands(root *cobra.Command, deps *Deps) {
	root.AddGroup(&cobra.Group{ID: GroupPlan, Title: "Plans (a human applies them):"})
	root.AddCommand(planCloseCommand(deps), planTidyCommand(deps), planInitCommand(deps), planApplyCommand(deps))
	commandParent(root, "digest", GroupRead).AddCommand(planDigestCommand(deps))
	commandParent(root, "milestone", GroupPlan).AddCommand(planMilestoneCreateCommand(deps), planMilestoneRetargetCommand(deps))
	commandParent(root, "plan", GroupRead).AddCommand(planListCommand(deps), planShowCommand(deps))
}
