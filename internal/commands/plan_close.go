package commands

import (
	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func planCloseCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use: "close <ref>", Short: "Plan closing an issue", GroupID: GroupPlan, Args: cobra.ExactArgs(1),
		Annotations: map[string]string{mcpArgsAnnotation: "ref: Issue reference: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pc, err := planStart(cmd.Context(), deps)
			if err != nil {
				return err
			}
			item, err := pc.planItem(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if item.Issue.State == domain.IssueClosed {
				return planMessage(deps, "Issue is already closed.")
			}
			// Plan files hold clean text; the title travels in the target and is
			// delimited when the plan is shown (section 6.8).
			description := "Fechar a issue " + item.Issue.Ref() + "."
			step := domain.Step{Operation: domain.OpCloseIssue, Target: planTarget(item), Before: string(item.Issue.State), After: string(domain.IssueClosed), Description: description}
			return pc.planSave("close "+item.Issue.Ref(), description, []domain.Step{step})
		},
	}
}
