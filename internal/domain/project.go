package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ProjectRef identifies a Projects v2 board by owner login and number. It is
// the form of the --project flag and of the project key of board.yml.
type ProjectRef struct {
	Owner  string `json:"owner"  yaml:"owner"`
	Number int    `json:"number" yaml:"number"`
}

// ParseProjectRef parses the "owner/number" form.
func ParseProjectRef(s string) (ProjectRef, error) {
	owner, number, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok || owner == "" || strings.ContainsAny(owner, "/# ") {
		return ProjectRef{}, fmt.Errorf("project %q: expected owner/number: %w", s, ErrUsage)
	}
	n, err := strconv.Atoi(number)
	if err != nil || n <= 0 {
		return ProjectRef{}, fmt.Errorf("project %q: number must be a positive integer: %w", s, ErrUsage)
	}
	return ProjectRef{Owner: owner, Number: n}, nil
}

// String returns the "owner/number" form.
func (r ProjectRef) String() string {
	return r.Owner + "/" + strconv.Itoa(r.Number)
}

// IsZero reports whether the reference is unset.
func (r ProjectRef) IsZero() bool { return r.Owner == "" && r.Number == 0 }

// Role is the viewer's role on a project as GitHub reports it.
type Role string

// Project roles.
const (
	RoleAdmin  Role = "ADMIN"
	RoleWriter Role = "WRITER"
	RoleReader Role = "READER"
	RoleNone   Role = "NONE"
)

// Project is a discovered board (section 5.1). Items are never part of it.
type Project struct {
	Ref    ProjectRef `json:"ref"`
	NodeID string     `json:"node_id"`
	Title  string     `json:"title"`
	// URL is the board page, which init writes into the forms' contact link.
	URL          string       `json:"url,omitempty"`
	README       string       `json:"readme"`
	Fields       []Field      `json:"fields"`
	Views        []View       `json:"views"`
	Workflows    []Workflow   `json:"workflows"`
	Repositories []Repository `json:"repositories"`
	ItemCount    int          `json:"item_count"`
	ViewerRole   Role         `json:"viewer_role"`
	DiscoveredAt time.Time    `json:"discovered_at"`
}

// FieldByName finds a field by its exact name.
func (p Project) FieldByName(name string) (Field, bool) {
	for _, f := range p.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// FieldNames lists the field names in discovery order.
func (p Project) FieldNames() []string {
	names := make([]string, 0, len(p.Fields))
	for _, f := range p.Fields {
		names = append(names, f.Name)
	}
	return names
}

// Repository is a repository linked to the project.
type Repository struct {
	NodeID string `json:"node_id"`
	Owner  string `json:"owner"`
	Name   string `json:"name"`
}

// FullName returns "owner/name".
func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

// View is a project view; the kit reads views and never edits them.
type View struct {
	NodeID string `json:"node_id"`
	Number int    `json:"number"`
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Filter string `json:"filter"`
}

// Workflow is a built-in project workflow with its enabled state.
type Workflow struct {
	NodeID  string `json:"node_id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// DataType is the ProjectV2FieldType of a field.
type DataType string

// Field data types as GitHub names them.
const (
	DataTypeText               DataType = "TEXT"
	DataTypeNumber             DataType = "NUMBER"
	DataTypeDate               DataType = "DATE"
	DataTypeSingleSelect       DataType = "SINGLE_SELECT"
	DataTypeIteration          DataType = "ITERATION"
	DataTypeTitle              DataType = "TITLE"
	DataTypeAssignees          DataType = "ASSIGNEES"
	DataTypeLabels             DataType = "LABELS"
	DataTypeMilestone          DataType = "MILESTONE"
	DataTypeRepository         DataType = "REPOSITORY"
	DataTypeLinkedPullRequests DataType = "LINKED_PULL_REQUESTS"
	DataTypeReviewers          DataType = "REVIEWERS"
	DataTypeTrackedBy          DataType = "TRACKED_BY"
	DataTypeTracks             DataType = "TRACKS"
	DataTypeSubIssuesProgress  DataType = "SUB_ISSUES_PROGRESS"
	DataTypeParentIssue        DataType = "PARENT_ISSUE"
	DataTypeIssueType          DataType = "ISSUE_TYPE"
)

// Field is a project field with its options or iterations.
type Field struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	DataType   DataType      `json:"data_type"`
	Options    []FieldOption `json:"options,omitempty"`
	Iterations []Iteration   `json:"iterations,omitempty"`
}

// OptionByName finds a single select option by its exact name.
func (f Field) OptionByName(name string) (FieldOption, bool) {
	for _, o := range f.Options {
		if o.Name == name {
			return o, true
		}
	}
	return FieldOption{}, false
}

// OptionNames lists the option names in board order.
func (f Field) OptionNames() []string {
	names := make([]string, 0, len(f.Options))
	for _, o := range f.Options {
		names = append(names, o.Name)
	}
	return names
}

// IterationByTitle finds an iteration by its exact title.
func (f Field) IterationByTitle(title string) (Iteration, bool) {
	for _, it := range f.Iterations {
		if it.Title == title {
			return it, true
		}
	}
	return Iteration{}, false
}

// IterationAt returns the iteration that contains the calendar date, if
// any; callers derive the date from an instant with Today.
func (f Field) IterationAt(day time.Time) (Iteration, bool) {
	for _, it := range f.Iterations {
		if it.Contains(day) {
			return it, true
		}
	}
	return Iteration{}, false
}

// FieldOption is one option of a single select field.
type FieldOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Iteration is one sprint of an iteration field. Start is a calendar
// date (midnight UTC, see dates.go) and Duration is in days.
type Iteration struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Start     time.Time `json:"start"`
	Duration  int       `json:"duration"`
	Completed bool      `json:"completed"`
}

// End is the first day after the iteration (start plus duration days).
func (i Iteration) End() time.Time { return i.Start.AddDate(0, 0, i.Duration) }

// LastDay is the last calendar date of the iteration, the end GitHub
// shows next to the title.
func (i Iteration) LastDay() time.Time { return i.End().AddDate(0, 0, -1) }

// Contains reports whether the calendar date falls inside the iteration.
func (i Iteration) Contains(day time.Time) bool {
	return !day.Before(i.Start) && day.Before(i.End())
}

// DaysLeft counts the days from the calendar date to the last day of the
// iteration: zero on the last day, never negative.
func (i Iteration) DaysLeft(day time.Time) int {
	left := int(math.Round(i.LastDay().Sub(CalendarDate(day)).Hours() / 24))
	if left < 0 {
		return 0
	}
	return left
}
