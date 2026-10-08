package github

import (
	"strconv"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Type names of the field value union members the adapter reads.
const (
	valueSingleSelect = "ProjectV2ItemFieldSingleSelectValue"
	valueIteration    = "ProjectV2ItemFieldIterationValue"
	valueDate         = "ProjectV2ItemFieldDateValue"
	valueText         = "ProjectV2ItemFieldTextValue"
	valueNumber       = "ProjectV2ItemFieldNumberValue"
	valueIssueField   = "ProjectV2ItemIssueFieldValue"

	issueValueDate         = "IssueFieldDateValue"
	issueValueSingleSelect = "IssueFieldSingleSelectValue"
	issueValueText         = "IssueFieldTextValue"
	issueValueNumber       = "IssueFieldNumberValue"
	issueValueMultiSelect  = "IssueFieldMultiSelectValue"

	contentIssue = "Issue"
)

// pageInfo is the cursor part of every connection the adapter pages.
type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type rawItemConnection struct {
	TotalCount int       `json:"totalCount"`
	PageInfo   pageInfo  `json:"pageInfo"`
	Nodes      []rawItem `json:"nodes"`
}

type rawItemList struct {
	Nodes []rawItem `json:"nodes"`
}

// rawItem is the ItemFields fragment.
type rawItem struct {
	ID         string `json:"id"`
	IsArchived bool   `json:"isArchived"`
	Type       string `json:"type"`
	Project    struct {
		ID string `json:"id"`
	} `json:"project"`
	FieldValues rawNodes[rawFieldValue] `json:"fieldValues"`
	Content     *rawIssue               `json:"content"`
}

type rawFieldValue struct {
	TypeName string `json:"__typename"`
	Field    struct {
		Name string `json:"name"`
	} `json:"field"`
	UpdatedAt       time.Time           `json:"updatedAt"`
	Creator         *rawUser            `json:"creator"`
	Name            string              `json:"name"`
	OptionID        string              `json:"optionId"`
	Title           string              `json:"title"`
	IterationID     string              `json:"iterationId"`
	Date            string              `json:"date"`
	Text            string              `json:"text"`
	Number          *float64            `json:"number"`
	IssueFieldValue *rawIssueFieldValue `json:"issueFieldValue"`
}

// rawIssueFieldValue is IssueFieldValueParts, also read through an item.
type rawIssueFieldValue struct {
	TypeName string `json:"__typename"`
	Field    struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"field"`
	DateValue   string   `json:"dateValue"`
	OptionName  string   `json:"optionName"`
	OptionID    string   `json:"optionId"`
	TextValue   string   `json:"textValue"`
	NumberValue *float64 `json:"numberValue"`
	MultiValue  string   `json:"multiValue"`
}

func (v rawIssueFieldValue) text() string {
	switch v.TypeName {
	case issueValueDate:
		return v.DateValue
	case issueValueSingleSelect:
		return v.OptionName
	case issueValueText:
		return v.TextValue
	case issueValueNumber:
		return formatNumber(v.NumberValue)
	case issueValueMultiSelect:
		return v.MultiValue
	default:
		return ""
	}
}

// toItem maps a project item whose content is an issue. Pull requests,
// draft issues and redacted items report false: the domain reads issues.
func toItem(raw rawItem) (domain.Item, bool) {
	if raw.Content == nil || raw.Content.TypeName != contentIssue {
		return domain.Item{}, false
	}
	it := domain.Item{
		ProjectItemID: raw.ID,
		Archived:      raw.IsArchived,
		Issue:         toIssue(*raw.Content),
		Values:        map[string]domain.FieldValue{},
	}
	for _, fv := range raw.FieldValues.Nodes {
		if value, ok := toFieldValue(fv); ok {
			it.Values[value.Field] = value
		}
	}
	if raw.Content.IssueFieldValues != nil {
		for _, v := range raw.Content.IssueFieldValues.Nodes {
			it.IssueFields = append(it.IssueFields, domain.IssueFieldValue{FieldID: v.Field.ID, Name: v.Field.Name, Value: v.text()})
		}
	}
	return it, true
}

// toFieldValue maps one field value; the members without a selection
// (users, labels, milestone, repository, reviewers) report false because
// the issue content already carries them.
func toFieldValue(fv rawFieldValue) (domain.FieldValue, bool) {
	if fv.Field.Name == "" {
		return domain.FieldValue{}, false
	}
	value := domain.FieldValue{Field: fv.Field.Name, UpdatedAt: fv.UpdatedAt}
	if fv.Creator != nil {
		value.Creator = fv.Creator.Login
	}
	switch fv.TypeName {
	case valueSingleSelect:
		value.Value, value.OptionID = fv.Name, fv.OptionID
	case valueIteration:
		value.Value, value.IterationID = fv.Title, fv.IterationID
	case valueDate:
		value.Value = fv.Date
	case valueText:
		value.Value = fv.Text
	case valueNumber:
		value.Value = formatNumber(fv.Number)
	case valueIssueField:
		if fv.IssueFieldValue == nil {
			return domain.FieldValue{}, false
		}
		value.Value, value.OptionID = fv.IssueFieldValue.text(), fv.IssueFieldValue.OptionID
	default:
		return domain.FieldValue{}, false
	}
	return value, true
}

func formatNumber(n *float64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatFloat(*n, 'f', -1, 64)
}
