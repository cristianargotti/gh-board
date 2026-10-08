package domain

import (
	"fmt"
	"strings"
	"time"
)

// Template declares what a template board carries besides its fields: the
// labels and the milestones init creates in the team repository when they
// are missing (section 10). It never edits what already exists.
type Template struct {
	Labels     []TemplateLabel     `yaml:"labels"     json:"labels"`
	Milestones []TemplateMilestone `yaml:"milestones" json:"milestones"`
}

// TemplateLabel is a label the template expects in the team repository;
// Color is six hex digits without the hash sign, as the API takes it.
type TemplateLabel struct {
	Name        string `yaml:"name"        json:"name"`
	Color       string `yaml:"color"       json:"color"`
	Description string `yaml:"description" json:"description"`
}

// TemplateMilestone is a milestone the template expects; DueOn is a
// YYYY-MM-DD date or empty.
type TemplateMilestone struct {
	Title       string `yaml:"title"       json:"title"`
	DueOn       string `yaml:"due_on"      json:"due_on"`
	Description string `yaml:"description" json:"description"`
}

// LabelInputs returns one createLabel input per template label for the
// repository; a nil template has none.
func (t *Template) LabelInputs(repositoryID string) []CreateLabelInput {
	if t == nil {
		return nil
	}
	out := make([]CreateLabelInput, 0, len(t.Labels))
	for _, l := range t.Labels {
		out = append(out, CreateLabelInput{RepositoryID: repositoryID, Name: l.Name, Color: l.Color, Description: l.Description})
	}
	return out
}

// MilestoneInputs returns one milestone input per template milestone for
// owner/repo, with the due date read as a calendar date.
func (t *Template) MilestoneInputs(owner, repo string, loc *time.Location) ([]CreateMilestoneInput, error) {
	if t == nil {
		return nil, nil
	}
	out := make([]CreateMilestoneInput, 0, len(t.Milestones))
	for _, m := range t.Milestones {
		in := CreateMilestoneInput{Owner: owner, Repo: repo, Title: m.Title, Description: m.Description}
		if m.DueOn != "" {
			due, ok := ParseDate(m.DueOn, loc)
			if !ok {
				return nil, fmt.Errorf("template milestone %q: due_on %q is not a YYYY-MM-DD date: %w", m.Title, m.DueOn, ErrUsage)
			}
			in.DueOn = &due
		}
		out = append(out, in)
	}
	return out, nil
}

// LabelNames lists the template label names, trimmed, in file order.
func (t *Template) LabelNames() []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, len(t.Labels))
	for _, l := range t.Labels {
		out = append(out, strings.TrimSpace(l.Name))
	}
	return out
}
