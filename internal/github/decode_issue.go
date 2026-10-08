package github

import (
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// rawIssue is the IssueFields fragment, also the content of an item.
type rawIssue struct {
	TypeName         string             `json:"__typename"`
	ID               string             `json:"id"`
	Number           int                `json:"number"`
	Title            string             `json:"title"`
	Body             string             `json:"body"`
	State            string             `json:"state"`
	URL              string             `json:"url"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
	ClosedAt         *time.Time         `json:"closedAt"`
	Repository       rawRepoRef         `json:"repository"`
	Labels           rawNodes[rawLabel] `json:"labels"`
	Assignees        rawNodes[rawUser]  `json:"assignees"`
	Milestone        *rawMilestone      `json:"milestone"`
	IssueType        *rawIssueType      `json:"issueType"`
	Parent           *rawParent         `json:"parent"`
	SubIssuesSummary struct {
		Total            int `json:"total"`
		Completed        int `json:"completed"`
		PercentCompleted int `json:"percentCompleted"`
	} `json:"subIssuesSummary"`
	BlockedBy struct {
		TotalCount int `json:"totalCount"`
	} `json:"blockedBy"`
	IssueFieldValues *rawNodes[rawIssueFieldValue] `json:"issueFieldValues"`
}

type rawRepoRef struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type rawParent struct {
	ID         string     `json:"id"`
	Number     int        `json:"number"`
	Title      string     `json:"title"`
	Repository rawRepoRef `json:"repository"`
}

type rawLabel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type rawMilestone struct {
	ID     string     `json:"id"`
	Number int        `json:"number"`
	Title  string     `json:"title"`
	State  string     `json:"state"`
	DueOn  *time.Time `json:"dueOn"`
}

type rawIssueType struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsEnabled bool   `json:"isEnabled"`
}

type rawComment struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	URL       string    `json:"url"`
	Author    *struct {
		Login string `json:"login"`
	} `json:"author"`
}

func toIssue(raw rawIssue) domain.Issue {
	issue := domain.Issue{
		NodeID:    raw.ID,
		Owner:     raw.Repository.Owner.Login,
		Repo:      raw.Repository.Name,
		Number:    raw.Number,
		Title:     raw.Title,
		Body:      raw.Body,
		State:     domain.IssueState(raw.State),
		URL:       raw.URL,
		Labels:    make([]domain.Label, 0, len(raw.Labels.Nodes)),
		Assignees: make([]domain.User, 0, len(raw.Assignees.Nodes)),
		SubIssues: domain.SubIssueSummary{
			Total:            raw.SubIssuesSummary.Total,
			Completed:        raw.SubIssuesSummary.Completed,
			PercentCompleted: raw.SubIssuesSummary.PercentCompleted,
		},
		BlockedByCount: raw.BlockedBy.TotalCount,
		CreatedAt:      raw.CreatedAt,
		UpdatedAt:      raw.UpdatedAt,
		ClosedAt:       raw.ClosedAt,
	}
	for _, l := range raw.Labels.Nodes {
		issue.Labels = append(issue.Labels, toLabel(l))
	}
	for _, u := range raw.Assignees.Nodes {
		issue.Assignees = append(issue.Assignees, domain.User{ID: u.ID, Login: u.Login})
	}
	if raw.Milestone != nil {
		m := toMilestone(*raw.Milestone)
		issue.Milestone = &m
	}
	if raw.IssueType != nil {
		issue.Type = &domain.IssueType{ID: raw.IssueType.ID, Name: raw.IssueType.Name}
	}
	if raw.Parent != nil {
		issue.Parent = toParent(*raw.Parent)
	}
	return issue
}

func toParent(raw rawParent) *domain.ParentRef {
	return &domain.ParentRef{
		NodeID: raw.ID,
		Owner:  raw.Repository.Owner.Login,
		Repo:   raw.Repository.Name,
		Number: raw.Number,
		Title:  raw.Title,
	}
}

func toLabel(raw rawLabel) domain.Label {
	return domain.Label{ID: raw.ID, Name: raw.Name, Color: raw.Color}
}

func toMilestone(raw rawMilestone) domain.Milestone {
	return domain.Milestone{ID: raw.ID, Number: raw.Number, Title: raw.Title, State: raw.State, DueOn: raw.DueOn}
}

func toComment(raw rawComment) domain.Comment {
	c := domain.Comment{ID: raw.ID, Body: raw.Body, CreatedAt: raw.CreatedAt, URL: raw.URL}
	if raw.Author != nil {
		c.Author = raw.Author.Login
	}
	return c
}
