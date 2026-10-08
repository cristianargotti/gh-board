package plan

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// setFieldValue writes a project field value: After is the human form,
// an option name, an iteration title, a number, a date or a text, and the
// discovered schema turns it into the id the mutation needs.
func (e *executor) setFieldValue(ctx context.Context, step domain.Step) (Outcome, error) {
	itemID, err := e.itemID(step)
	if err != nil {
		return Outcome{}, err
	}
	field, err := domain.ResolveField(e.snap.Project, step.Field)
	if err != nil {
		return Outcome{}, err
	}
	in := domain.ItemFieldValueInput{ProjectID: e.projectID(), ItemID: itemID, FieldID: field.ID}
	if err := fieldInput(&in, field, step.After); err != nil {
		return Outcome{}, err
	}
	return finish(e.ports.Writer.UpdateItemFieldValue(ctx, in))
}

// fieldInput fills the one value member the field type takes.
func fieldInput(in *domain.ItemFieldValueInput, field domain.Field, value string) error {
	switch field.DataType {
	case domain.DataTypeSingleSelect:
		opt, err := domain.ResolveOption(field, value)
		if err != nil {
			return err
		}
		in.SingleSelectID = &opt.ID
	case domain.DataTypeIteration:
		it, err := domain.ResolveIteration(field, value)
		if err != nil {
			return err
		}
		in.IterationID = &it.ID
	case domain.DataTypeNumber:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("field %q takes a number, got %q: %w", field.Name, value, domain.ErrUsage)
		}
		in.Number = &n
	case domain.DataTypeDate:
		t, ok := domain.ParseDate(value, time.UTC)
		if !ok {
			return fmt.Errorf("field %q takes a date as YYYY-MM-DD, got %q: %w", field.Name, value, domain.ErrUsage)
		}
		in.Date = &t
	case domain.DataTypeText:
		text := value
		in.Text = &text
	default:
		return fmt.Errorf("field %q of type %s cannot be set by the kit: %w", field.Name, field.DataType, domain.ErrUsage)
	}
	return nil
}

// setIssueFieldValue writes an organization issue field: the existing
// value is updated, a first value is set through the field id the
// resolver returns.
func (e *executor) setIssueFieldValue(ctx context.Context, step domain.Step) (Outcome, error) {
	issueID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	if it, ok := e.item(step); ok {
		if v, found := it.IssueField(step.Field); found && v.FieldID != "" {
			in := domain.IssueFieldValueInput{IssueID: issueID, FieldID: v.FieldID, Value: step.After}
			return finish(e.ports.Writer.UpdateIssueFieldValue(ctx, in))
		}
	}
	resolver, ok := e.ports.Reader.(IssueFieldResolver)
	if !ok {
		return Outcome{}, fmt.Errorf("issue field %q: the adapter cannot resolve organization issue fields yet: %w", step.Field, domain.ErrNotImplemented)
	}
	owner, _ := splitRepo(step.Target.Repository)
	fieldID, err := resolver.IssueFieldID(ctx, owner, step.Field)
	if err != nil {
		return Outcome{}, err
	}
	in := domain.IssueFieldValueInput{IssueID: issueID, FieldID: fieldID, Value: step.After}
	return finish(e.ports.Writer.SetIssueFieldValue(ctx, in))
}
