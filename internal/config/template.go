package config

import (
	"regexp"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// labelColor is the six hex digit color the createLabel mutation takes,
// without the hash sign.
var labelColor = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

// checkTemplate rejects template declarations init could not create: a
// label without a name or with a color GitHub refuses, a milestone
// without a title or with a due date that is not YYYY-MM-DD, and names
// declared twice, which GitHub treats as the same label or milestone.
func checkTemplate(cfg domain.Config) error {
	t := cfg.Template
	if t == nil {
		return nil
	}
	if err := checkTemplateLabels(t.Labels); err != nil {
		return err
	}
	return checkTemplateMilestones(t.Milestones)
}

func checkTemplateLabels(labels []domain.TemplateLabel) error {
	seen := map[string]bool{}
	for i, l := range labels {
		name := strings.TrimSpace(l.Name)
		if name == "" {
			return usage("template.labels[%d].name is required", i)
		}
		if seen[strings.ToLower(name)] {
			return usage("template.labels lists %q twice", name)
		}
		seen[strings.ToLower(name)] = true
		if !labelColor.MatchString(l.Color) {
			return usage("template.labels[%d].color must be six hex digits without the hash sign, got %q", i, l.Color)
		}
	}
	return nil
}

func checkTemplateMilestones(milestones []domain.TemplateMilestone) error {
	seen := map[string]bool{}
	for i, m := range milestones {
		title := strings.TrimSpace(m.Title)
		if title == "" {
			return usage("template.milestones[%d].title is required", i)
		}
		if seen[strings.ToLower(title)] {
			return usage("template.milestones lists %q twice", title)
		}
		seen[strings.ToLower(title)] = true
		if m.DueOn != "" {
			if _, err := time.Parse(domain.DateLayout, m.DueOn); err != nil {
				return usage("template.milestones[%d].due_on must be YYYY-MM-DD, got %q", i, m.DueOn)
			}
		}
	}
	return nil
}
