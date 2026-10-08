package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Fields of an update_issue step, the dimension its Before value reads.
const (
	UpdateFieldTitle     = "title"
	UpdateFieldBody      = "body"
	UpdateFieldMilestone = "milestone"
)

// Values a step stores for the issue state and the item archive state.
const (
	ArchivedValue = "archived"
	ActiveValue   = "active"
)

// Drift is one precondition that changed since the plan was written.
type Drift struct {
	Step     int    `json:"step"`
	Target   Target `json:"target"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Missing  bool   `json:"missing"`
}

// String describes the drift for the terminal.
func (d Drift) String() string {
	where := fmt.Sprintf("step %d %s#%d", d.Step, d.Target.Repository, d.Target.Number)
	if d.Field != "" {
		where += " " + d.Field
	}
	if d.Missing {
		return where + ": target could not be read"
	}
	return fmt.Sprintf("%s: expected %q, found %q", where, d.Expected, d.Actual)
}

// DetectDrift compares the Before value of every step with the items
// re-read right before apply, keyed by node id. It reports every drift,
// not only the first, so the human sees the whole picture.
func DetectDrift(p Plan, current map[string]Item) []Drift {
	var drifts []Drift
	for _, step := range p.Steps {
		if !HasPrecondition(step.Operation) {
			continue
		}
		it, ok := current[step.Target.NodeID]
		if !ok {
			drifts = append(drifts, Drift{Step: step.Index, Target: step.Target, Field: step.Field, Expected: step.Before, Missing: true})
			continue
		}
		actual, _ := StepCurrentValue(step, it)
		if actual != step.Before {
			drifts = append(drifts, Drift{Step: step.Index, Target: step.Target, Field: step.Field, Expected: step.Before, Actual: actual})
		}
	}
	return drifts
}

// DriftError wraps the drifts as an ErrDrift error (exit code 4), or
// returns nil when there is none.
func DriftError(drifts []Drift) error {
	if len(drifts) == 0 {
		return nil
	}
	lines := make([]string, 0, len(drifts))
	for _, d := range drifts {
		lines = append(lines, d.String())
	}
	return fmt.Errorf("%d precondition(s) changed since the plan was written: %s: %w", len(drifts), strings.Join(lines, "; "), ErrDrift)
}

// HasPrecondition reports whether an operation carries a Before value that
// apply must re-read. Creations and comments carry none.
func HasPrecondition(op Operation) bool {
	switch op {
	case OpCloseIssue, OpReopenIssue, OpSetFieldValue, OpSetIssueFieldValue,
		OpAddAssignees, OpRemoveAssignees, OpAddLabels, OpRemoveLabels,
		OpUpdateIssue, OpSetIssueType, OpAddSubIssue, OpUnarchiveItem:
		return true
	default:
		return false
	}
}

// StepCurrentValue reads from the item the value a step's Before describes,
// encoded the way plan builders must encode it. ok is false when the
// operation carries no precondition.
func StepCurrentValue(step Step, it Item) (string, bool) {
	switch step.Operation {
	case OpSetFieldValue:
		return it.Text(step.Field), true
	case OpSetIssueFieldValue:
		v, _ := it.IssueField(step.Field)
		return v.Value, true
	case OpCloseIssue, OpReopenIssue:
		return string(it.Issue.State), true
	case OpAddAssignees, OpRemoveAssignees:
		return EncodeAssignees(it.Issue.Assignees), true
	case OpAddLabels, OpRemoveLabels:
		return EncodeLabels(it.Issue.Labels), true
	case OpUpdateIssue:
		return issueField(it.Issue, step.Field), true
	case OpSetIssueType:
		return issueTypeName(it.Issue), true
	case OpAddSubIssue:
		return parentRef(it.Issue), true
	case OpUnarchiveItem:
		return archiveState(it), true
	default:
		return "", false
	}
}

func issueField(i Issue, field string) string {
	switch field {
	case UpdateFieldTitle:
		return i.Title
	case UpdateFieldBody:
		return i.Body
	case UpdateFieldMilestone:
		if i.Milestone != nil {
			return i.Milestone.Title
		}
	}
	return ""
}

func issueTypeName(i Issue) string {
	if i.Type == nil {
		return ""
	}
	return i.Type.Name
}

func parentRef(i Issue) string {
	if i.Parent == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s#%d", i.Parent.Owner, i.Parent.Repo, i.Parent.Number)
}

func archiveState(it Item) string {
	if it.Archived {
		return ArchivedValue
	}
	return ActiveValue
}

// EncodeAssignees is the canonical Before form of an assignee set: logins
// sorted and joined by comma.
func EncodeAssignees(users []User) string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Login)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// EncodeLabels is the canonical Before form of a label set: names sorted
// and joined by comma.
func EncodeLabels(labels []Label) string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}
