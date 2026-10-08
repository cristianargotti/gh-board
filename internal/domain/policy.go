package domain

import (
	"fmt"
	"sort"
	"strings"
)

// PolicyKind names the team rule that refused a write.
type PolicyKind string

// Refusal kinds. Every one maps to exit code 3 through ErrPolicy.
const (
	PolicyTransition PolicyKind = "transition"
	PolicyWIP        PolicyKind = "wip"
	PolicyPermission PolicyKind = "permission"
)

// PolicyError is a write refused by the team process. Reason is in PT-BR
// because the team reads it on the board and through the agents.
type PolicyError struct {
	Kind   PolicyKind `json:"kind"`
	Status string     `json:"status,omitempty"`
	From   string     `json:"from,omitempty"`
	To     string     `json:"to,omitempty"`
	Reason string     `json:"reason"`
}

// Error joins the English kind with the PT-BR reason.
func (e *PolicyError) Error() string {
	return fmt.Sprintf("refused by %s policy: %s", e.Kind, e.Reason)
}

// Unwrap maps every refusal to ErrPolicy.
func (e *PolicyError) Unwrap() error { return ErrPolicy }

// PolicyOf returns the team policy, empty in generic mode (nil config).
func PolicyOf(cfg *Config) Policy {
	if cfg == nil {
		return Policy{}
	}
	return cfg.Policy
}

// AllowedTransitions lists the statuses reachable from one status. The
// second result is false when the policy declares no rule for the status.
func (p Policy) AllowedTransitions(from string) ([]string, bool) {
	targets, ok := p.Transitions[from]
	return targets, ok
}

// CheckTransition validates a status move. A source status without a rule
// allows every destination; a rule lists the only destinations allowed.
// Staying in the same status is not a transition and is never refused.
func CheckTransition(p Policy, from, to string) error {
	targets, ok := p.AllowedTransitions(from)
	if from == to || !ok || Contains(targets, to) {
		return nil
	}
	allowed := "nenhuma"
	if len(targets) > 0 {
		allowed = strings.Join(targets, ", ")
	}
	return &PolicyError{
		Kind: PolicyTransition, From: from, To: to,
		Reason: fmt.Sprintf("transição de %s para %s não permitida; permitidas: %s", from, to, allowed),
	}
}

// WIPLimit returns the limit configured for a status, if any.
func (p Policy) WIPLimit(status string) (int, bool) {
	limit, ok := p.WIP[status]
	return limit, ok && limit > 0
}

// CheckWIP refuses a move into a status when the open items already there
// fill the limit. occupied excludes the item being moved.
func CheckWIP(p Policy, status string, occupied int) error {
	limit, ok := p.WIPLimit(status)
	if !ok || occupied < limit {
		return nil
	}
	return &PolicyError{
		Kind: PolicyWIP, Status: status,
		Reason: fmt.Sprintf("limite de WIP em %s atingido: %d de %d", status, occupied, limit),
	}
}

// CheckMove runs the transition rule and then the WIP rule of a move.
func CheckMove(p Policy, from, to string, occupied int) error {
	if err := CheckTransition(p, from, to); err != nil {
		return err
	}
	return CheckWIP(p, to, occupied)
}

// CountByStatus counts the open, unarchived items per value of a field.
func CountByStatus(items []Item, field string) map[string]int {
	counts := make(map[string]int)
	for _, it := range Workable(items) {
		if v := it.Text(field); v != "" {
			counts[v]++
		}
	}
	return counts
}

// WIPOccupied counts the open, unarchived items in a status, leaving out
// the item identified by excludeNodeID: the occupancy CheckWIP expects when
// that item is the one being moved.
func WIPOccupied(items []Item, field, status, excludeNodeID string) int {
	n := 0
	for _, it := range Workable(items) {
		if it.Issue.NodeID != excludeNodeID && it.Text(field) == status {
			n++
		}
	}
	return n
}

// WIPExcess is a status over its limit.
type WIPExcess struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
	Limit  int    `json:"limit"`
}

// Exceeded lists the statuses over their WIP limit, sorted by name.
func (p Policy) Exceeded(counts map[string]int) []WIPExcess {
	var out []WIPExcess
	for status, limit := range p.WIP {
		if limit > 0 && counts[status] > limit {
			out = append(out, WIPExcess{Status: status, Count: counts[status], Limit: limit})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Status < out[j].Status })
	return out
}

// RequiresPlan reports whether a direct write over n items exceeds the bulk
// threshold and must become a plan (section 6.3). Zero disables the rule.
func (p Policy) RequiresPlan(n int) bool {
	return p.BulkThreshold > 0 && n > p.BulkThreshold
}

// CheckPermission refuses a write when the viewer cannot write to the
// repository that holds the issue.
func CheckPermission(perm Permission, repository string) error {
	if perm.CanWrite() {
		return nil
	}
	current := string(perm)
	if current == "" {
		current = string(PermissionNone)
	}
	return &PolicyError{
		Kind:   PolicyPermission,
		Reason: fmt.Sprintf("sem permissão de escrita em %s (permissão atual: %s)", repository, current),
	}
}
