package domain

import "time"

// Viewer is the effective identity behind the token (section 6.6).
type Viewer struct {
	Login string `json:"login"`
	ID    string `json:"id"`
	Host  string `json:"host"`
	// TokenSource is what go-gh reports: GH_TOKEN, GITHUB_TOKEN, oauth_token
	// (config file), gh (keyring) or default (no token).
	TokenSource string `json:"token_source"`
}

// Shadowed reports whether an environment token overrides the keyring.
func (v Viewer) Shadowed() bool {
	return v.TokenSource == "GH_TOKEN" || v.TokenSource == "GITHUB_TOKEN"
}

// Permission is the viewer's permission on a repository.
type Permission string

// Repository permissions as GitHub names them.
const (
	PermissionAdmin    Permission = "ADMIN"
	PermissionMaintain Permission = "MAINTAIN"
	PermissionWrite    Permission = "WRITE"
	PermissionTriage   Permission = "TRIAGE"
	PermissionRead     Permission = "READ"
	PermissionNone     Permission = "NONE"
)

// CanWrite reports whether the permission allows issue writes.
func (p Permission) CanWrite() bool {
	switch p {
	case PermissionAdmin, PermissionMaintain, PermissionWrite, PermissionTriage:
		return true
	default:
		return false
	}
}

// ListOptions controls pagination, filtering and the selection of
// ListItems.
type ListOptions struct {
	Cursor    string
	Limit     int
	All       bool
	Filter    ItemFilter
	Selection ItemSelection
}

// ItemSelection says how much of each item a listing reads. item, search,
// roadmap, digest and every write read everything; the listings that only
// summarize items skip the body, the milestone and the parent of the
// issue and the issue field values surfaced as project fields, which
// makes a page of items lighter and faster (section 7).
type ItemSelection string

// Item selections of ListOptions; the zero value reads everything.
const (
	SelectionFull    ItemSelection = ""
	SelectionSummary ItemSelection = "summary"
)

// ItemFilter is the filter set of the list command (section 7). Empty
// values mean "no filter"; names are human values, not ids.
type ItemFilter struct {
	Status   string
	Assignee string
	Lane     string
	Epic     string
	Sprint   string
	Label    string
	Type     string
	State    IssueState
	Overdue  bool
	Blocked  bool
	Triage   bool
	Archived bool
}

// ItemPage is one page of items. Skipped counts the board items of the
// page that are not issues (draft issues, pull requests, redacted items),
// which the kit leaves out and reports as omitted (section 6.8).
type ItemPage struct {
	Items      []Item
	NextCursor string
	HasNext    bool
	TotalCount int
	Skipped    int
}

// Comment is an issue comment from the timeline.
type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url"`
}

// CreateIssueInput is the input of createIssue. The JSON names are what a
// plan step carries in After and what plan show prints.
type CreateIssueInput struct {
	RepositoryID string   `json:"repository_id"`
	Title        string   `json:"title"`
	Body         string   `json:"body,omitempty"`
	IssueTypeID  string   `json:"issue_type_id,omitempty"`
	MilestoneID  string   `json:"milestone_id,omitempty"`
	AssigneeIDs  []string `json:"assignee_ids,omitempty"`
	LabelIDs     []string `json:"label_ids,omitempty"`
}

// UpdateIssueInput is the input of updateIssue restricted to title, body
// and milestone. A nil pointer leaves the attribute untouched.
type UpdateIssueInput struct {
	IssueID     string
	Title       *string
	Body        *string
	MilestoneID *string
}

// IssueFieldValueInput targets an organization issue field value.
type IssueFieldValueInput struct {
	IssueID string
	FieldID string
	// Value is the textual form; dates use YYYY-MM-DD.
	Value string
}

// ItemFieldValueInput is the input of updateProjectV2ItemFieldValue.
// Exactly one of the value members is set.
type ItemFieldValueInput struct {
	ProjectID      string
	ItemID         string
	FieldID        string
	Text           *string
	Number         *float64
	Date           *time.Time
	SingleSelectID *string
	IterationID    *string
}

// StatusUpdateInput is the input of createProjectV2StatusUpdate.
type StatusUpdateInput struct {
	ProjectID  string
	Body       string
	Status     string
	StartDate  *time.Time
	TargetDate *time.Time
}

// CreateFieldInput is the input of createProjectV2Field.
type CreateFieldInput struct {
	ProjectID           string   `json:"project_id"`
	Name                string   `json:"name"`
	DataType            DataType `json:"data_type"`
	SingleSelectOptions []string `json:"single_select_options,omitempty"`
}

// CreateLabelInput is the input of createLabel.
type CreateLabelInput struct {
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	Color        string `json:"color,omitempty"`
	Description  string `json:"description,omitempty"`
}

// CreateMilestoneInput is the body of POST /repos/{owner}/{repo}/milestones.
// DueOn is a calendar date (dates.go).
type CreateMilestoneInput struct {
	Owner       string     `json:"owner"`
	Repo        string     `json:"repo"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	DueOn       *time.Time `json:"due_on,omitempty"`
}

// CopyProjectInput is the input of copyProjectV2.
type CopyProjectInput struct {
	SourceProjectID    string `json:"source_project_id"`
	OwnerID            string `json:"owner_id"`
	Title              string `json:"title"`
	IncludeDraftIssues bool   `json:"include_draft_issues"`
}
