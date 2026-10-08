package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// CreateMilestone creates a milestone through POST
// /repos/{owner}/{repo}/milestones, the only REST write of the kit,
// because GraphQL has no createMilestone (plans only). The state comes
// back in the GraphQL spelling so both readers agree.
func (a *Adapter) CreateMilestone(ctx context.Context, in domain.CreateMilestoneInput) (domain.Milestone, error) {
	if in.Owner == "" || in.Repo == "" || in.Title == "" {
		return domain.Milestone{}, usage("create_milestone: owner, repository and title are required")
	}
	body := map[string]any{varTitle: in.Title}
	if in.Description != "" {
		body["description"] = in.Description
	}
	if in.DueOn != nil {
		// GitHub keeps the calendar date of the instant in US Pacific time, so
		// midnight UTC lands on the day before; noon UTC is the same date in
		// every zone GitHub could apply.
		due := in.DueOn.UTC()
		body["due_on"] = time.Date(due.Year(), due.Month(), due.Day(), 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
	var out struct {
		NodeID string     `json:"node_id"`
		Number int        `json:"number"`
		Title  string     `json:"title"`
		State  string     `json:"state"`
		DueOn  *time.Time `json:"due_on"`
	}
	path := fmt.Sprintf("repos/%s/%s/milestones", url.PathEscape(in.Owner), url.PathEscape(in.Repo))
	if err := a.post(ctx, path, body, &out); err != nil {
		return domain.Milestone{}, err
	}
	return domain.Milestone{
		ID:     out.NodeID,
		Number: out.Number,
		Title:  out.Title,
		State:  strings.ToUpper(out.State),
		DueOn:  out.DueOn,
	}, nil
}
