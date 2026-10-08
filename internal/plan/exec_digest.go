package plan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func (e *executor) createStatusUpdate(ctx context.Context, step domain.Step) (Outcome, error) {
	var in domain.StatusUpdateInput
	if err := decodePayload(step, &in); err != nil {
		return Outcome{}, err
	}
	projectID, err := e.resolve(in.ProjectID)
	if err != nil {
		return Outcome{}, err
	}
	in.ProjectID = projectID
	if in.ProjectID == "" {
		in.ProjectID = e.projectID()
	}
	lister, ok := e.ports.Reader.(StatusUpdateLister)
	if !ok {
		return Outcome{}, fmt.Errorf("status update history is required for weekly idempotency: %w", domain.ErrAPI)
	}
	updates, err := lister.ListStatusUpdates(ctx, in.ProjectID)
	if err != nil {
		return Outcome{}, err
	}
	if in.StartDate == nil {
		in.StartDate = &e.plan.CreatedAt
	}
	if u, found := sameWeek(updates, *in.StartDate); found {
		return skipped(u.ID, "this ISO week already has a status update")
	}
	in.Body = markedBody(in.Body, WeekMarker(*in.StartDate))
	id, err := e.ports.Writer.CreateStatusUpdate(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	return created(id, "status update posted")
}

// The payload's reporting week survives retries across ISO year boundaries.
func sameWeek(updates []StatusUpdate, at time.Time) (StatusUpdate, bool) {
	marker := WeekMarker(at)
	for _, u := range updates {
		if SameISOWeek(u.CreatedAt, at) || strings.Contains(u.Body, marker) {
			return u, true
		}
	}
	return StatusUpdate{}, false
}
