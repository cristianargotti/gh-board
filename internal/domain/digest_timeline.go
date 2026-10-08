package domain

import (
	"context"
	"time"
)

// DecisionEvent records a decision's actual author and time; the current field
// value alone cannot establish the first response in an issue's history.
type DecisionEvent struct {
	ID        string    `json:"id"`
	Field     string    `json:"field"`
	Value     string    `json:"value"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url"`
}

// IssueTimeline distinguishes missing history from an observed empty history.
// Complete covers both comments and decisions through the digest's Now.
type IssueTimeline struct {
	Comments  []Comment       `json:"comments"`
	Decisions []DecisionEvent `json:"decisions"`
	Complete  bool            `json:"complete"`
	Reason    string          `json:"reason,omitempty"`
}

// IssueTimelineReader is the narrow read port needed to prove first response.
// Adapters must not mark comment-only or truncated history as complete.
type IssueTimelineReader interface {
	IssueTimeline(ctx context.Context, issueID string) (IssueTimeline, error)
}
