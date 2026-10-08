package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// PlanExpiry is how long a plan stays applicable after creation (6.4).
const PlanExpiry = 30 * time.Minute

// Operation names what a plan step does. The names are the kit's verbs, not
// GraphQL mutation names, because effects decide the tier (principle 3).
type Operation string

// Plan step operations. Each maps to exactly one allowlisted mutation of
// section 6.2 in internal/plan.
const (
	OpCloseIssue         Operation = "close_issue"
	OpReopenIssue        Operation = "reopen_issue"
	OpSetFieldValue      Operation = "set_field_value"
	OpSetIssueFieldValue Operation = "set_issue_field_value"
	OpAddAssignees       Operation = "add_assignees"
	OpRemoveAssignees    Operation = "remove_assignees"
	OpAddLabels          Operation = "add_labels"
	OpRemoveLabels       Operation = "remove_labels"
	OpAddComment         Operation = "add_comment"
	OpCreateIssue        Operation = "create_issue"
	OpUpdateIssue        Operation = "update_issue"
	OpAddSubIssue        Operation = "add_sub_issue"
	OpSetIssueType       Operation = "set_issue_type"
	OpAddProjectItem     Operation = "add_project_item"
	OpUnarchiveItem      Operation = "unarchive_item"
	OpCreateStatusUpdate Operation = "create_status_update"
	OpCreateField        Operation = "create_field"
	OpCreateLabel        Operation = "create_label"
	OpCreateMilestone    Operation = "create_milestone"
	OpCopyProject        Operation = "copy_project"
	OpLinkRepository     Operation = "link_repository"
)

// Plan is the immutable record a guarded operation writes and a human
// applies (section 6.4, ADR-003). Description is in PT-BR for the team.
type Plan struct {
	ID          string     `json:"id"`
	KitVersion  string     `json:"kit_version"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Actor       string     `json:"actor"`
	Host        string     `json:"host"`
	Project     ProjectRef `json:"project"`
	ProjectID   string     `json:"project_id"`
	Command     string     `json:"command"`
	Description string     `json:"description"`
	Steps       []Step     `json:"steps"`
	Hash        string     `json:"hash"`
}

// Step is one write of a plan with the value read at creation (Before) and
// the value it sets (After); apply refuses when Before drifted.
type Step struct {
	Index       int       `json:"index"`
	Operation   Operation `json:"operation"`
	Target      Target    `json:"target"`
	Field       string    `json:"field,omitempty"`
	Before      string    `json:"before"`
	After       string    `json:"after"`
	Description string    `json:"description"`
}

// Target names the issue or item a step touches, by node id and for humans.
type Target struct {
	NodeID        string `json:"node_id"`
	ProjectItemID string `json:"project_item_id,omitempty"`
	Repository    string `json:"repository"`
	Number        int    `json:"number"`
	Title         string `json:"title"`
}

// ComputeHash returns the SHA-256 of the canonical JSON of the plan with
// the Hash field empty. Write stores it; apply recomputes and compares.
func (p Plan) ComputeHash() (string, error) {
	p.Hash = ""
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// Expired reports whether the plan is past its expiry at the instant.
func (p Plan) Expired(now time.Time) bool { return !now.Before(p.ExpiresAt) }

// Items lists the targets of the plan as owner/repo#n, in step order,
// without duplicates; targets without a number (a repository, a new
// project) are named by their repository or title.
func (p Plan) Items() []string {
	seen := make(map[string]bool, len(p.Steps))
	refs := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		ref := s.Target.Ref()
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	return refs
}

// Ref names the target for a reader: owner/repo#n when it is an issue,
// else the repository or the title.
func (t Target) Ref() string {
	switch {
	case t.Repository != "" && t.Number > 0:
		return fmt.Sprintf("%s#%d", t.Repository, t.Number)
	case t.Repository != "":
		return t.Repository
	default:
		return CleanTitle(t.Title)
	}
}

// Targets lists the node ids the plan touches, in step order, without
// duplicates.
func (p Plan) Targets() []string {
	seen := make(map[string]bool, len(p.Steps))
	ids := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		if s.Target.NodeID == "" || seen[s.Target.NodeID] {
			continue
		}
		seen[s.Target.NodeID] = true
		ids = append(ids, s.Target.NodeID)
	}
	return ids
}
