package commands

import (
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func (s *writeSession) writeResolve(raw string) (domain.Item, error) {
	ref, err := domain.ParseReference(raw)
	if err != nil {
		return domain.Item{}, err
	}
	ref, err = domain.ResolveReference(ref, s.cfg.Repository)
	if err != nil {
		return domain.Item{}, err
	}
	it, err := s.deps.Reader.GetItem(s.ctx, s.project, ref)
	if err != nil {
		return it, writeAPI(err)
	}
	if it.Issue.NodeID == "" {
		return it, domain.Errorf(domain.ExitNotFound, "target has no issue node ID")
	}
	if err := s.writePermission(it.Issue.Owner, it.Issue.Repo); err != nil {
		return it, err
	}
	return it, nil
}

func (s *writeSession) writeTargets(raw string, build func(domain.Item) error) error {
	for _, ref := range strings.Split(raw, ",") {
		it, err := s.writeResolve(ref)
		if err != nil {
			return err
		}
		if _, ok := s.items[it.Issue.NodeID]; ok {
			continue
		}
		s.items[it.Issue.NodeID] = it
		if err := s.writeExpectations(it); err != nil {
			return err
		}
		if err := build(it); err != nil {
			return err
		}
	}
	s.guarded = s.guarded || s.cfg.Policy.RequiresPlan(len(s.items))
	return nil
}

func writeTarget(it domain.Item) domain.Target {
	return domain.Target{
		NodeID: it.Issue.NodeID, ProjectItemID: it.ProjectItemID,
		Repository: it.Issue.Owner + "/" + it.Issue.Repo, Number: it.Issue.Number, Title: it.Issue.Title,
	}
}

func (s *writeSession) writeAdd(it domain.Item, op domain.Operation, field, after, description string, run writeFuncItem) {
	step := domain.Step{
		Index: len(s.actions), Operation: op, Target: writeTarget(it),
		Field: field, After: after, Description: description,
	}
	step.Before, _ = plan.Precondition(step, it)
	s.actions = append(s.actions, writeAction{step: step, run: run})
}

func (s *writeSession) writeExpectations(it domain.Item) error {
	values := map[string]string{}
	for _, raw := range s.deps.Flags.Expect {
		field, value, err := writePair(raw)
		if err != nil {
			return err
		}
		if old, ok := values[field]; ok && old != value {
			return domain.Errorf(domain.ExitUsage, "conflicting expectations for %q", field)
		}
		values[field] = value
	}
	s.expect[it.Issue.NodeID] = values
	return s.writeCheckExpect(it)
}

func (s *writeSession) writeCheckExpect(it domain.Item) error {
	for field, want := range s.expect[it.Issue.NodeID] {
		got, err := writeExpectedValue(s.cfg, s.project, it, field)
		if err != nil {
			return err
		}
		if got != want {
			return domain.Errorf(domain.ExitDrift, "%s %s: expected %q, found %q", it.Issue.Ref(), field, want, got)
		}
	}
	return nil
}

func writeExpectedValue(cfg *domain.Config, project domain.Project, it domain.Item, field string) (string, error) {
	step, err := writeCondition(cfg, project, it, field, "")
	if err != nil {
		return "", err
	}
	value, _ := plan.Precondition(step, it)
	return value, nil
}
