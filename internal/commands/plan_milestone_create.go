package commands

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func planMilestoneCreateCommand(deps *Deps) *cobra.Command {
	var due, description string
	cmd := &cobra.Command{
		Use: "create <title>", Short: "Plan creating a repository milestone", GroupID: GroupPlan, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pc, err := planStart(cmd.Context(), deps)
			if err != nil {
				return err
			}
			if err := pc.planPermission(cmd.Context(), pc.cfg.Repository); err != nil {
				return err
			}
			input, err := planMilestoneInput(pc.cfg.Repository, args[0], description, due)
			if err != nil {
				return err
			}
			milestones, err := deps.Reader.RepositoryMilestones(cmd.Context(), input.Owner, input.Repo)
			if err != nil {
				return planAPI(err)
			}
			if _, err := domain.ResolveMilestone(milestones, input.Title); err == nil {
				return planMessage(deps, "Milestone already exists.")
			}
			payload, err := planJSON(input)
			if err != nil {
				return err
			}
			text := "Criar o marco " + domain.CleanTitle(input.Title) + " em " + pc.cfg.Repository + "."
			step := domain.Step{Operation: domain.OpCreateMilestone, Target: domain.Target{Repository: pc.cfg.Repository, Title: input.Title}, After: payload, Description: text}
			return pc.planSave("milestone create", text, []domain.Step{step})
		},
	}
	cmd.Flags().StringVar(&due, "due", "", "due date as YYYY-MM-DD")
	cmd.Flags().StringVar(&description, "description", "", "milestone description")
	return cmd
}

func planMilestoneInput(repository, title, description, due string) (domain.CreateMilestoneInput, error) {
	owner, repo, err := domain.SplitRepository(repository)
	input := domain.CreateMilestoneInput{Owner: owner, Repo: repo, Title: title, Description: description}
	if err != nil {
		return input, err
	}
	if strings.TrimSpace(title) == "" {
		return input, domain.Errorf(domain.ExitUsage, "milestone title must not be empty")
	}
	if due != "" {
		date, err := time.Parse(time.DateOnly, due)
		if err != nil {
			return input, domain.Errorf(domain.ExitUsage, "--due requires YYYY-MM-DD")
		}
		input.DueOn = &date
	}
	return input, nil
}
