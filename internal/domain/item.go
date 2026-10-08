package domain

import (
	"strconv"
	"strings"
	"time"
)

// IssueState is the open or closed state of an issue.
type IssueState string

// Issue states as GitHub names them.
const (
	IssueOpen   IssueState = "OPEN"
	IssueClosed IssueState = "CLOSED"
)

// Item is a project item together with the issue it points to. Items are
// read fresh on every command and never cached (section 5.1).
type Item struct {
	ProjectItemID string                `json:"project_item_id"`
	Archived      bool                  `json:"archived"`
	Issue         Issue                 `json:"issue"`
	Values        map[string]FieldValue `json:"values"`
	IssueFields   []IssueFieldValue     `json:"issue_fields,omitempty"`
}

// Issue is the issue content behind an item.
type Issue struct {
	NodeID         string          `json:"node_id"`
	Owner          string          `json:"owner"`
	Repo           string          `json:"repo"`
	Number         int             `json:"number"`
	Title          string          `json:"title"`
	Body           string          `json:"body"`
	State          IssueState      `json:"state"`
	URL            string          `json:"url"`
	Labels         []Label         `json:"labels"`
	Assignees      []User          `json:"assignees"`
	Milestone      *Milestone      `json:"milestone,omitempty"`
	Type           *IssueType      `json:"type,omitempty"`
	Parent         *ParentRef      `json:"parent,omitempty"`
	SubIssues      SubIssueSummary `json:"sub_issues"`
	BlockedByCount int             `json:"blocked_by_count"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	ClosedAt       *time.Time      `json:"closed_at,omitempty"`
}

// Ref returns the "owner/repo#number" form of the issue.
func (i Issue) Ref() string {
	return i.Owner + "/" + i.Repo + "#" + strconv.Itoa(i.Number)
}

// Label is a repository label.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// User is a GitHub account by login.
type User struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

// Milestone is a repository milestone.
type Milestone struct {
	ID     string     `json:"id"`
	Number int        `json:"number"`
	Title  string     `json:"title"`
	State  string     `json:"state"`
	DueOn  *time.Time `json:"due_on,omitempty"`
}

// IssueType is an organization issue type (Feature, Task, Bug...).
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ParentRef points to the parent issue of a sub-issue.
type ParentRef struct {
	NodeID string `json:"node_id"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// SubIssueSummary is the sub-issue progress GitHub computes for a parent.
type SubIssueSummary struct {
	Total            int `json:"total"`
	Completed        int `json:"completed"`
	PercentCompleted int `json:"percent_completed"`
}

// FieldValue is the value of one project field on an item. Value is the
// human form (option name, iteration title, text, number or date); the ids
// are kept for writes and preconditions. UpdatedAt changes only when this
// value is edited, which stale_active relies on.
type FieldValue struct {
	Field       string    `json:"field"`
	Value       string    `json:"value"`
	OptionID    string    `json:"option_id,omitempty"`
	IterationID string    `json:"iteration_id,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	Creator     string    `json:"creator,omitempty"`
}

// IssueFieldValue is an organization issue field value read through
// Issue.issueFieldValues (dates.source issue_fields).
type IssueFieldValue struct {
	FieldID string `json:"field_id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

// Value returns the project field value by field name.
func (it Item) Value(field string) (FieldValue, bool) {
	v, ok := it.Values[field]
	return v, ok
}

// Text returns the human value of a field, or the empty string.
func (it Item) Text(field string) string {
	return it.Values[field].Value
}

// IssueField returns an organization issue field value by name.
func (it Item) IssueField(name string) (IssueFieldValue, bool) {
	for _, v := range it.IssueFields {
		if v.Name == name {
			return v, true
		}
	}
	return IssueFieldValue{}, false
}

// HasLabel reports whether the issue carries the label, ignoring case.
func (it Item) HasLabel(name string) bool {
	for _, l := range it.Issue.Labels {
		if strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}

// AssignedTo reports whether the login is an assignee, ignoring case.
func (it Item) AssignedTo(login string) bool {
	for _, u := range it.Issue.Assignees {
		if strings.EqualFold(u.Login, login) {
			return true
		}
	}
	return false
}

// IsOpen reports whether the issue is open.
func (it Item) IsOpen() bool { return it.Issue.State == IssueOpen }
