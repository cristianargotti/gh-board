package commands

import (
	"sort"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func writeCondition(cfg *domain.Config, project domain.Project, it domain.Item, name, value string) (domain.Step, error) {
	step := domain.Step{
		Target: writeTarget(it), Field: name, Before: value, After: value,
		Description: "Verificar condição informada, sem alterar o valor.",
	}
	dates := cfg.Capabilities.Dates
	if dates != nil && dates.Source == domain.DateSourceIssueFields && (name == dates.Start || name == dates.Target) {
		step.Operation = domain.OpSetIssueFieldValue
		return step, nil
	}
	if field, err := domain.ResolveField(project, name); err == nil {
		step.Operation, step.Field = writeFieldCondition(field)
		return step, nil
	}
	if _, ok := it.IssueField(name); ok {
		step.Operation = domain.OpSetIssueFieldValue
		return step, nil
	}
	step.Field = strings.ToLower(name)
	step.Operation = writeBuiltinCondition(step.Field)
	if step.Operation == "" {
		return step, domain.NotFound("field", name, project.FieldNames())
	}
	return step, nil
}

func writeFieldCondition(field domain.Field) (domain.Operation, string) {
	switch field.DataType {
	case domain.DataTypeAssignees:
		return domain.OpAddAssignees, "assignees"
	case domain.DataTypeLabels:
		return domain.OpAddLabels, "labels"
	case domain.DataTypeMilestone:
		return domain.OpUpdateIssue, domain.UpdateFieldMilestone
	case domain.DataTypeTitle:
		return domain.OpUpdateIssue, domain.UpdateFieldTitle
	case domain.DataTypeParentIssue:
		return domain.OpAddSubIssue, "parent"
	case domain.DataTypeIssueType:
		return domain.OpSetIssueType, "type"
	default:
		return domain.OpSetFieldValue, field.Name
	}
}

func writeBuiltinCondition(field string) domain.Operation {
	operations := map[string]domain.Operation{
		"state": domain.OpReopenIssue, "archived": domain.OpUnarchiveItem,
		"assignees": domain.OpAddAssignees, "labels": domain.OpAddLabels, "parent": domain.OpAddSubIssue,
		"milestone": domain.OpUpdateIssue, "title": domain.OpUpdateIssue, "body": domain.OpUpdateIssue, "type": domain.OpSetIssueType,
	}
	return operations[field]
}

func (s *writeSession) writePlanConditions() error {
	if !s.guarded {
		return nil
	}
	ids := make([]string, 0, len(s.expect))
	for id := range s.expect {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := s.writeItemConditions(s.items[id]); err != nil {
			return err
		}
	}
	return nil
}

func (s *writeSession) writeItemConditions(it domain.Item) error {
	names := make([]string, 0, len(s.expect[it.Issue.NodeID]))
	for name := range s.expect[it.Issue.NodeID] {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		step, err := writeCondition(s.cfg, s.project, it, name, s.expect[it.Issue.NodeID][name])
		if err != nil {
			return err
		}
		if writeHasCondition(s.plan.Steps, step) {
			continue
		}
		step.Index = len(s.plan.Steps)
		s.plan.Steps = append(s.plan.Steps, step)
	}
	return nil
}

func writeHasCondition(steps []domain.Step, condition domain.Step) bool {
	for _, step := range steps {
		if step.Target.NodeID == condition.Target.NodeID && step.Field == condition.Field &&
			writeConditionOperation(step.Operation) == writeConditionOperation(condition.Operation) {
			return true
		}
	}
	return false
}

func writeConditionOperation(operation domain.Operation) domain.Operation {
	switch operation {
	case domain.OpRemoveLabels:
		return domain.OpAddLabels
	case domain.OpRemoveAssignees:
		return domain.OpAddAssignees
	default:
		return operation
	}
}
