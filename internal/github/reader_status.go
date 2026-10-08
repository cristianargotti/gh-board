package github

import (
	"context"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// statusUpdatePageSize is the page of the status updates connection.
const statusUpdatePageSize = 50

type rawStatusUpdate struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	StartDate  string    `json:"startDate"`
	TargetDate string    `json:"targetDate"`
	Creator    *struct {
		Login string `json:"login"`
	} `json:"creator"`
}

// ListStatusUpdates lists the status updates of a project, newest first,
// so digest post can skip an ISO week that already has one.
func (a *Adapter) ListStatusUpdates(ctx context.Context, projectID string) ([]domain.StatusUpdate, error) {
	if projectID == "" {
		return nil, usage("status_updates: a project id is required")
	}
	updates := []domain.StatusUpdate{}
	base := map[string]any{varID: projectID}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Node *struct {
				StatusUpdates *struct {
					PageInfo pageInfo          `json:"pageInfo"`
					Nodes    []rawStatusUpdate `json:"nodes"`
				} `json:"statusUpdates"`
			} `json:"node"`
		}
		if err := a.run(ctx, "status_updates", pageVariables(base, statusUpdatePageSize, after), &out); err != nil {
			return pageInfo{}, err
		}
		if out.Node == nil || out.Node.StatusUpdates == nil {
			return pageInfo{}, notFound("status_updates", "project "+projectID)
		}
		for _, raw := range out.Node.StatusUpdates.Nodes {
			updates = append(updates, toStatusUpdate(raw))
		}
		return out.Node.StatusUpdates.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func toStatusUpdate(raw rawStatusUpdate) domain.StatusUpdate {
	u := domain.StatusUpdate{ID: raw.ID, Body: raw.Body, Status: raw.Status, CreatedAt: raw.CreatedAt}
	if raw.Creator != nil {
		u.Creator = raw.Creator.Login
	}
	u.StartDate = parseDay(raw.StartDate)
	u.TargetDate = parseDay(raw.TargetDate)
	return u
}

// parseDay reads a Date scalar; an empty or malformed value is nil.
func parseDay(s string) *time.Time {
	t, err := time.Parse(domain.DateLayout, s)
	if err != nil {
		return nil
	}
	return &t
}
