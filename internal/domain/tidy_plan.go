package domain

import (
	"fmt"
	"time"
)

// TidyInput supplies the complete item set and discovered schema for planning.
type TidyInput struct {
	Config  *Config
	Project Project
	Items   []Item
	Now     time.Time
}

// TidySkip explains why a requested routine could not safely produce a step.
type TidySkip struct {
	Item   ItemRef `json:"item"`
	Rule   string  `json:"rule"`
	Reason string  `json:"reason"`
}

// TidyResult contains only proposed writes; the command seals and persists them.
type TidyResult struct {
	Steps   []Step     `json:"steps"`
	Skipped []TidySkip `json:"skipped"`
}

type tidyPlanner struct {
	in     TidyInput
	caps   Capabilities
	loc    *time.Location
	result TidyResult
	counts map[string]int
}

// BuildTidyPlan computes drift-checkable steps without changing the input.
// Ambiguous mappings and incomplete evidence are reported instead of guessed.
func BuildTidyPlan(in TidyInput) TidyResult {
	p := tidyPlanner{
		in: in, caps: CapabilitiesOf(in.Config), loc: LocationOf(in.Config),
		result: TidyResult{Steps: []Step{}, Skipped: []TidySkip{}}, counts: CountByStatus(in.Items, StatusFieldName(in.Config)),
	}
	if in.Config == nil {
		return p.result
	}
	for _, it := range Workable(in.Items) {
		p.inherit(it)
		if in.Config.Tidy.SprintFromTarget {
			p.sprint(it)
		}
		if in.Config.Tidy.StampStartOnActive {
			p.start(it)
		}
		if in.Config.Tidy.EpicFollowsTasks && IsEpic(p.caps, it) {
			p.epic(it)
		}
	}
	return p.result
}

func (p *tidyPlanner) skip(it Item, rule, reason string) {
	p.result.Skipped = append(p.result.Skipped, TidySkip{Item: ItemRefOf(it), Rule: rule, Reason: reason})
}

func (p *tidyPlanner) step(it Item, op Operation, field, before, after, description string) {
	if after == "" || before == after {
		return
	}
	target := Target{
		NodeID: it.Issue.NodeID, ProjectItemID: it.ProjectItemID,
		Repository: it.Issue.Owner + "/" + it.Issue.Repo, Number: it.Issue.Number, Title: CleanTitle(it.Issue.Title),
	}
	p.result.Steps = append(p.result.Steps, Step{
		Index: len(p.result.Steps), Operation: op,
		Target: target, Field: field, Before: before, After: after, Description: description,
	})
}

func (p *tidyPlanner) projectStep(it Item, field, after, rule, why string) bool {
	if after == "" || it.Text(field) == after {
		return false
	}
	f, ok := p.in.Project.FieldByName(field)
	if !ok || !tidyAccepts(f, after) {
		p.skip(it, rule, "campo ou valor ausente no esquema: "+CleanTitle(field))
		return false
	}
	description := fmt.Sprintf("%s: %s = %s", why, CleanTitle(field), CleanTitle(after))
	p.step(it, OpSetFieldValue, field, it.Text(field), after, description)
	return true
}

func tidyAccepts(field Field, value string) bool {
	switch field.DataType {
	case DataTypeSingleSelect:
		_, ok := field.OptionByName(value)
		return ok
	case DataTypeIteration:
		_, ok := field.IterationByTitle(value)
		return ok
	case DataTypeText, DataTypeDate:
		return true
	default:
		return false
	}
}
