package domain

import (
	"fmt"
	"time"
)

// DigestInput supplies a fresh board snapshot and complete timelines by issue
// node ID. Week is an ISO week, or empty for the week containing Now.
type DigestInput struct {
	Config     *Config
	Project    Project
	Items      []Item
	Now        time.Time
	Week       string
	Timelines  map[string]IssueTimeline
	Milestones []DigestMilestone
}

// DigestResult keeps the evidence next to the PT-BR text for JSON consumers.
type DigestResult struct {
	Week        string            `json:"week"`
	Start       time.Time         `json:"start"`
	End         time.Time         `json:"end"`
	GeneratedAt time.Time         `json:"generated_at"`
	Project     ProjectRef        `json:"project"`
	Deliveries  []ItemSummary     `json:"deliveries"`
	Milestones  []DigestMilestone `json:"milestones"`
	Risks       []Alert           `json:"risks"`
	RisksAsOf   time.Time         `json:"risks_as_of"`
	Metrics     []DigestMetric    `json:"metrics"`
	Text        string            `json:"text"`
}

// DigestMilestone scopes milestones to repositories, where their numbers live.
type DigestMilestone struct {
	Repository string    `json:"repository"`
	Milestone  Milestone `json:"milestone"`
}

// BuildDigest computes a weekly digest without reading or writing external state.
func BuildDigest(in DigestInput) (DigestResult, error) {
	if in.Now.IsZero() {
		return DigestResult{}, fmt.Errorf("digest: now is required: %w", ErrUsage)
	}
	start, err := digestWeekStart(in.Week, in.Now, LocationOf(in.Config))
	if err != nil {
		return DigestResult{}, err
	}
	year, week := start.ISOWeek()
	out := DigestResult{
		Week: fmt.Sprintf("%04d-W%02d", year, week), Start: start, End: start.AddDate(0, 0, 7),
		GeneratedAt: in.Now, Project: in.Project.Ref, RisksAsOf: in.Now,
	}
	out.Deliveries = SummarizeItems(in.Config, digestClosed(in.Items, out.Start, out.End, in.Now))
	out.Milestones = digestMilestones(in, out)
	out.Risks = EvaluateAlerts(AlertInput{Config: in.Config, Project: in.Project, Items: in.Items, Now: in.Now})
	out.Metrics = digestMetrics(in, out)
	out.Text = digestText(out)
	return out, nil
}

func digestClosed(items []Item, start, end, now time.Time) []Item {
	out := []Item{}
	for _, it := range items {
		at := it.Issue.ClosedAt
		if it.Issue.State == IssueClosed && at != nil && digestInWeek(*at, start, end) && !at.After(now) {
			out = append(out, it)
		}
	}
	return out
}

func digestInWeek(at, start, end time.Time) bool {
	return !at.IsZero() && !at.Before(start) && at.Before(end)
}

// digestMilestones lists the milestones due in the week. Due dates are
// calendar dates, so they are compared with the calendar dates of the
// week bounds, not with the instants.
func digestMilestones(in DigestInput, out DigestResult) []DigestMilestone {
	candidates := append([]DigestMilestone{}, in.Milestones...)
	for _, it := range in.Items {
		if it.Issue.Milestone != nil {
			candidates = append(candidates, DigestMilestone{it.Issue.Owner + "/" + it.Issue.Repo, *it.Issue.Milestone})
		}
	}
	loc := LocationOf(in.Config)
	first, last := Today(out.Start, loc), Today(out.End, loc)
	seen := map[string]bool{}
	result := []DigestMilestone{}
	for _, entry := range candidates {
		m := entry.Milestone
		key := fmt.Sprintf("%s#%d:%s", entry.Repository, m.Number, m.ID)
		if seen[key] || m.DueOn == nil || !digestInWeek(CalendarDate(*m.DueOn), first, last) {
			continue
		}
		seen[key] = true
		entry.Milestone.Title = CleanTitle(m.Title)
		result = append(result, entry)
	}
	return result
}
