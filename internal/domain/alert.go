package domain

import "time"

// Alert rule ids of section 9. They are stable: the mod and alerts.json
// consumers rely on them.
const (
	AlertOverdue          = "overdue"
	AlertBlocked          = "blocked"
	AlertTriageSLA        = "triage_sla"
	AlertWIPExceeded      = "wip_exceeded"
	AlertSprintEnding     = "sprint_ending"
	AlertStaleActive      = "stale_active"
	AlertEpicWithoutDates = "epic_without_dates"
)

// Severity orders alerts; an alert that gains severity is "escalated".
type Severity string

// Severities from least to most urgent.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Rank orders severities for escalation comparisons.
func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// ItemRef is the minimal item identity an alert carries.
type ItemRef struct {
	NodeID        string `json:"node_id"`
	ProjectItemID string `json:"project_item_id,omitempty"`
	Repository    string `json:"repository"`
	Number        int    `json:"number"`
	Title         string `json:"title"`
	URL           string `json:"url"`
}

// Alert is one finding of an alert rule. Message is in PT-BR.
type Alert struct {
	RuleID    string    `json:"rule_id"`
	Severity  Severity  `json:"severity"`
	Item      ItemRef   `json:"item"`
	Message   string    `json:"message"`
	FirstSeen time.Time `json:"first_seen"`
}

// Fingerprint identifies an alert across evaluations: rule plus item.
// Board-level alerts (WIP exceeded, sprint ending) carry an empty node id.
func (a Alert) Fingerprint() string { return a.RuleID + "|" + a.Item.NodeID }

// AlertState is the alerts.json contract the Claude Code mod reads: the
// mod shows Summary in the status line and toasts new alerts.
type AlertState struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Project     ProjectRef `json:"project"`
	Summary     string     `json:"summary"`
	Alerts      []Alert    `json:"alerts"`
}

// Find returns the alert with the fingerprint, if present.
func (s AlertState) Find(fingerprint string) (Alert, bool) {
	for _, a := range s.Alerts {
		if a.Fingerprint() == fingerprint {
			return a, true
		}
	}
	return Alert{}, false
}

// CountByRule counts alerts per rule id.
func (s AlertState) CountByRule() map[string]int {
	counts := make(map[string]int)
	for _, a := range s.Alerts {
		counts[a.RuleID]++
	}
	return counts
}
