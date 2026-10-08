package github

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// timelinePageSize is the page of the timeline items connection.
const timelinePageSize = 100

// Timeline item kinds the document selects.
const (
	timelineComment      = "IssueComment"
	timelineFieldAdded   = "IssueFieldAddedEvent"
	timelineFieldChanged = "IssueFieldChangedEvent"
)

type rawActor struct {
	Login string `json:"login"`
}

// rawTimelineItem is one node of IssueTimeline: a comment, or an
// organization issue field event with the value it set.
type rawTimelineItem struct {
	TypeName   string    `json:"__typename"`
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	URL        string    `json:"url"`
	Author     *rawActor `json:"author"`
	Actor      *rawActor `json:"actor"`
	IssueField *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"issueField"`
	NewValue   string `json:"newValue"`
	NewOptions []struct {
		Name string `json:"name"`
	} `json:"newOptions"`
}

// IssueTimeline reads the comments and the organization issue field events
// of an issue, oldest first, then adds one decision per project field
// value the issue carries on its boards: GitHub keeps no timeline event
// for a project field, so its latest value with its creator and updatedAt
// stands for the decision. Complete is true once every page was read.
func (a *Adapter) IssueTimeline(ctx context.Context, issueID string) (domain.IssueTimeline, error) {
	if issueID == "" {
		return domain.IssueTimeline{}, usage("issue_timeline: an issue id is required")
	}
	tl := domain.IssueTimeline{Comments: []domain.Comment{}, Decisions: []domain.DecisionEvent{}}
	base := map[string]any{varID: issueID}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Node *struct {
				TimelineItems *struct {
					PageInfo pageInfo          `json:"pageInfo"`
					Nodes    []rawTimelineItem `json:"nodes"`
				} `json:"timelineItems"`
			} `json:"node"`
		}
		if err := a.run(ctx, "issue_timeline", pageVariables(base, timelinePageSize, after), &out); err != nil {
			return pageInfo{}, err
		}
		if out.Node == nil || out.Node.TimelineItems == nil {
			return pageInfo{}, notFound("issue_timeline", "issue "+issueID)
		}
		for _, raw := range out.Node.TimelineItems.Nodes {
			addTimelineItem(&tl, raw)
		}
		return out.Node.TimelineItems.PageInfo, nil
	})
	if err != nil {
		return domain.IssueTimeline{}, err
	}
	decisions, err := a.projectDecisions(ctx, issueID)
	if err != nil {
		return domain.IssueTimeline{}, err
	}
	tl.Decisions = append(tl.Decisions, decisions...)
	tl.Complete = true
	return tl, nil
}

func addTimelineItem(tl *domain.IssueTimeline, raw rawTimelineItem) {
	switch raw.TypeName {
	case timelineComment:
		c := domain.Comment{ID: raw.ID, Body: raw.Body, CreatedAt: raw.CreatedAt, URL: raw.URL}
		if raw.Author != nil {
			c.Author = raw.Author.Login
		}
		tl.Comments = append(tl.Comments, c)
	case timelineFieldAdded, timelineFieldChanged:
		if raw.IssueField == nil {
			return
		}
		d := domain.DecisionEvent{ID: raw.ID, Field: raw.IssueField.Name, Value: eventValue(raw), CreatedAt: raw.CreatedAt}
		if raw.Actor != nil {
			d.Actor = raw.Actor.Login
		}
		tl.Decisions = append(tl.Decisions, d)
	}
}

// eventValue is the value an issue field event set: the scalar, or the
// option names of a select field joined by commas.
func eventValue(raw rawTimelineItem) string {
	if raw.NewValue != "" {
		return raw.NewValue
	}
	names := make([]string, 0, len(raw.NewOptions))
	for _, o := range raw.NewOptions {
		names = append(names, o.Name)
	}
	return strings.Join(names, ", ")
}

// projectDecisions reads the issue's items on every board and turns each
// field value that carries a creator into a decision event dated by the
// value's updatedAt, in field name order.
func (a *Adapter) projectDecisions(ctx context.Context, issueID string) ([]domain.DecisionEvent, error) {
	var out struct {
		Node *rawItemNode `json:"node"`
	}
	if err := a.run(ctx, "item_by_node", map[string]any{varID: issueID}, &out); err != nil {
		return nil, err
	}
	if out.Node == nil || out.Node.TypeName != nodeIssue || out.Node.ProjectItems == nil {
		return nil, nil
	}
	var events []domain.DecisionEvent
	for _, raw := range out.Node.ProjectItems.Nodes {
		it, ok := toItem(raw)
		if !ok {
			continue
		}
		names := make([]string, 0, len(it.Values))
		for name := range it.Values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			v := it.Values[name]
			if v.Creator == "" || v.UpdatedAt.IsZero() {
				continue
			}
			events = append(events, domain.DecisionEvent{ID: it.ProjectItemID + "/" + name, Field: name, Value: v.Value, Actor: v.Creator, CreatedAt: v.UpdatedAt})
		}
	}
	return events, nil
}
