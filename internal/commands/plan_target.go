package commands

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func (pc *planContext) planItem(ctx context.Context, raw string) (domain.Item, error) {
	ref, err := domain.ParseReference(raw)
	if err != nil {
		return domain.Item{}, err
	}
	ref, err = domain.ResolveReference(ref, pc.cfg.Repository)
	if err != nil {
		return domain.Item{}, err
	}
	return pc.planGetItem(ctx, ref)
}

func (pc *planContext) planGetItem(ctx context.Context, ref domain.Reference) (domain.Item, error) {
	item, err := pc.deps.Reader.GetItem(ctx, pc.project, ref)
	if err != nil {
		return item, planAPI(err)
	}
	if item.Issue.NodeID == "" {
		return item, domain.Errorf(domain.ExitNotFound, "target has no issue node id")
	}
	if err := pc.planPermission(ctx, item.Issue.Owner+"/"+item.Issue.Repo); err != nil {
		return item, err
	}
	return item, planExpect(item, pc.deps.Flags.Expect)
}

func (pc *planContext) planPermission(ctx context.Context, repository string) error {
	owner, repo, err := domain.SplitRepository(repository)
	if err != nil {
		return err
	}
	permission, err := pc.deps.Reader.ViewerPermission(ctx, owner, repo)
	if err != nil {
		return planAPI(err)
	}
	return domain.CheckPermission(permission, repository)
}

func planExpect(item domain.Item, expectations []string) error {
	for _, expression := range expectations {
		field, want, ok := strings.Cut(expression, "=")
		if !ok || field == "" {
			return domain.Errorf(domain.ExitUsage, "--expect requires field=value")
		}
		got, exists := planExpectedValue(item, field)
		if !exists {
			return domain.Errorf(domain.ExitNotFound, "expectation field %s not found", render.Title(field))
		}
		if got != want {
			return domain.Errorf(domain.ExitDrift, "%s: expected %s, found %s", render.Title(field), render.Excerpt(render.LabelBody, want), render.Excerpt(render.LabelBody, got))
		}
	}
	return nil
}

func planExpectedValue(item domain.Item, field string) (string, bool) {
	switch field {
	case "state":
		return string(item.Issue.State), true
	case domain.UpdateFieldMilestone, domain.UpdateFieldTitle, domain.UpdateFieldBody:
		return domain.StepCurrentValue(domain.Step{Operation: domain.OpUpdateIssue, Field: field}, item)
	}
	if value, ok := item.Value(field); ok {
		return value.Value, true
	}
	if value, ok := item.IssueField(field); ok {
		return value.Value, true
	}
	return "", false
}

func planTarget(item domain.Item) domain.Target {
	return domain.Target{
		NodeID: item.Issue.NodeID, ProjectItemID: item.ProjectItemID,
		Repository: item.Issue.Owner + "/" + item.Issue.Repo, Number: item.Issue.Number, Title: item.Issue.Title,
	}
}
