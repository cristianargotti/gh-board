package github

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Issue field kinds GitHub reports (IssueFieldDataType).
const (
	issueFieldText         = "TEXT"
	issueFieldSingleSelect = "SINGLE_SELECT"
	issueFieldDate         = "DATE"
	issueFieldNumber       = "NUMBER"
	issueFieldMultiSelect  = "MULTI_SELECT"
)

// rawIssueField is the IssueField answer: the kind and the options.
type rawIssueField struct {
	TypeName string      `json:"__typename"`
	Name     string      `json:"name"`
	DataType string      `json:"dataType"`
	Options  []rawOption `json:"options"`
}

// SetIssueFieldValue runs setIssueFieldValue (organization issue field,
// first value).
func (a *Adapter) SetIssueFieldValue(ctx context.Context, in domain.IssueFieldValueInput) error {
	input, err := a.issueFieldInput(ctx, in)
	if err != nil {
		return err
	}
	var out struct{}
	vars := map[string]any{varIssueID: in.IssueID, "issueFields": []map[string]any{input}}
	return a.run(ctx, "set_issue_field_value", vars, &out)
}

// UpdateIssueFieldValue runs updateIssueFieldValue (existing value).
func (a *Adapter) UpdateIssueFieldValue(ctx context.Context, in domain.IssueFieldValueInput) error {
	input, err := a.issueFieldInput(ctx, in)
	if err != nil {
		return err
	}
	var out struct{}
	return a.run(ctx, "update_issue_field_value", map[string]any{varIssueID: in.IssueID, "issueField": input}, &out)
}

// issueFieldInput builds IssueFieldCreateOrUpdateInput. The field kind is
// read fresh so the value lands in the member of its type; the delete
// member is never set.
func (a *Adapter) issueFieldInput(ctx context.Context, in domain.IssueFieldValueInput) (map[string]any, error) {
	if in.IssueID == "" || in.FieldID == "" || in.Value == "" {
		return nil, usage("issue field value: issue id, field id and a value are required, the kit never clears a value")
	}
	var out struct {
		Node *rawIssueField `json:"node"`
	}
	if err := a.run(ctx, "issue_field", map[string]any{varID: in.FieldID}, &out); err != nil {
		return nil, err
	}
	if out.Node == nil || out.Node.DataType == "" {
		return nil, notFound("issue_field", "issue field "+in.FieldID)
	}
	value, err := issueFieldMember(out.Node, in.Value)
	if err != nil {
		return nil, err
	}
	return map[string]any{"fieldId": in.FieldID, value.name: value.value}, nil
}

type member struct {
	name  string
	value any
}

func issueFieldMember(field *rawIssueField, text string) (member, error) {
	switch field.DataType {
	case issueFieldText:
		return member{"textValue", text}, nil
	case issueFieldDate:
		if _, err := time.Parse(domain.DateLayout, text); err != nil {
			return member{}, usage("issue field %s: %q is not a YYYY-MM-DD date", field.Name, text)
		}
		return member{"dateValue", text}, nil
	case issueFieldNumber:
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return member{}, usage("issue field %s: %q is not a number", field.Name, text)
		}
		return member{"numberValue", n}, nil
	case issueFieldSingleSelect:
		id, err := optionID(field, text)
		return member{"singleSelectOptionId", id}, err
	case issueFieldMultiSelect:
		ids, err := optionIDs(field, text)
		return member{"multiSelectOptionIds", ids}, err
	default:
		return member{}, usage("issue field %s has the unsupported kind %s", field.Name, field.DataType)
	}
}

// optionID resolves an option by id or by name, ignoring case.
func optionID(field *rawIssueField, text string) (string, error) {
	names := make([]string, 0, len(field.Options))
	for _, o := range field.Options {
		if o.ID == text || strings.EqualFold(o.Name, text) {
			return o.ID, nil
		}
		names = append(names, o.Name)
	}
	return "", domain.NotFound("option of "+field.Name, text, names)
}

// optionIDs resolves a comma separated list of options.
func optionIDs(field *rawIssueField, text string) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(text, ",") {
		id, err := optionID(field, strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// UpdateItemFieldValue runs updateProjectV2ItemFieldValue.
func (a *Adapter) UpdateItemFieldValue(ctx context.Context, in domain.ItemFieldValueInput) error {
	if in.ProjectID == "" || in.ItemID == "" || in.FieldID == "" {
		return usage("item field value: project, item and field ids are required")
	}
	value, err := fieldValue(in)
	if err != nil {
		return err
	}
	vars := map[string]any{varProjectID: in.ProjectID, "itemId": in.ItemID, "fieldId": in.FieldID, "value": value}
	var out struct{}
	return a.run(ctx, "update_item_field_value", vars, &out)
}

// fieldValue builds ProjectV2FieldValue from exactly one member, so that the
// kit never sends an empty value, which would clear the field.
func fieldValue(in domain.ItemFieldValueInput) (map[string]any, error) {
	value := map[string]any{}
	if in.Text != nil && *in.Text != "" {
		value["text"] = *in.Text
	}
	if in.Number != nil {
		value["number"] = *in.Number
	}
	if in.Date != nil {
		value["date"] = in.Date.Format(domain.DateLayout)
	}
	if in.SingleSelectID != nil && *in.SingleSelectID != "" {
		value["singleSelectOptionId"] = *in.SingleSelectID
	}
	if in.IterationID != nil && *in.IterationID != "" {
		value["iterationId"] = *in.IterationID
	}
	if len(value) != 1 {
		return nil, usage("item field %s: exactly one non-empty value member is required, got %d", in.FieldID, len(value))
	}
	return value, nil
}
