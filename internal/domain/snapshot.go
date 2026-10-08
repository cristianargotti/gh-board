package domain

import "time"

// Completeness says whether the snapshot fit the budget.
type Completeness string

// Completeness values of the context command (F10).
const (
	Complete Completeness = "complete"
	Partial  Completeness = "partial"
)

// Snapshot sections in truncation order (section 7, context).
const (
	SectionRules      = "rules"
	SectionSprint     = "sprint"
	SectionMine       = "mine"
	SectionAttention  = "attention"
	SectionTriage     = "triage"
	SectionEpics      = "epics"
	SectionDeliveries = "deliveries"
)

// SnapshotSections lists the sections in the order the budget drops them
// from the end.
var SnapshotSections = []string{
	SectionRules, SectionSprint, SectionMine, SectionAttention,
	SectionTriage, SectionEpics, SectionDeliveries,
}

// DefaultContextBudget is the approximate token budget of context.
const DefaultContextBudget = 1500

// Snapshot is the result of gh board context: what an agent needs to start.
type Snapshot struct {
	GeneratedAt  time.Time      `json:"generated_at"`
	Project      ProjectRef     `json:"project"`
	Title        string         `json:"title"`
	Viewer       string         `json:"viewer"`
	Rules        TeamRules      `json:"rules"`
	Sprint       *SprintSummary `json:"sprint,omitempty"`
	Mine         []ItemSummary  `json:"mine"`
	Attention    []Alert        `json:"attention"`
	Triage       []ItemSummary  `json:"triage"`
	Epics        []EpicProgress `json:"epics"`
	Deliveries   []ItemSummary  `json:"deliveries"`
	Completeness Completeness   `json:"completeness"`
	Omitted      map[string]int `json:"omitted"`
}

// TeamRules is the compact form of the team process for a prompt.
type TeamRules struct {
	Flow     []string `json:"flow"`
	WIP      []string `json:"wip"`
	SLA      []string `json:"sla"`
	Rituals  []string `json:"rituals"`
	Language string   `json:"language"`
}

// SprintSummary is the current iteration with days left. Start and End
// are calendar dates (YYYY-MM-DD); End is the last day of the sprint, the
// one GitHub shows, and DaysLeft is zero on that day.
type SprintSummary struct {
	Title    string `json:"title"`
	Start    string `json:"start"`
	End      string `json:"end"`
	DaysLeft int    `json:"days_left"`
	Open     int    `json:"open"`
	Done     int    `json:"done"`
}

// ItemSummary is an item reduced to what a prompt needs; Title is already
// sanitized and truncated (section 6.8).
type ItemSummary struct {
	Ref       string   `json:"ref"`
	NodeID    string   `json:"node_id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Assignees []string `json:"assignees"`
	Lane      string   `json:"lane,omitempty"`
	Epic      string   `json:"epic,omitempty"`
	Sprint    string   `json:"sprint,omitempty"`
	Target    string   `json:"target,omitempty"`
	URL       string   `json:"url"`
}

// EpicProgress is an epic with its sub-issue progress and dates.
type EpicProgress struct {
	Ref       string `json:"ref"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Owner     string `json:"owner"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Start     string `json:"start,omitempty"`
	Target    string `json:"target,omitempty"`
}

// MarkOmitted records that n entries of a section were dropped by the
// budget and flags the snapshot as partial.
func (s *Snapshot) MarkOmitted(section string, n int) {
	if n <= 0 {
		return
	}
	if s.Omitted == nil {
		s.Omitted = make(map[string]int)
	}
	s.Omitted[section] += n
	s.Completeness = Partial
}

// OmittedTotal sums the omitted counts of every section.
func (s Snapshot) OmittedTotal() int {
	total := 0
	for _, n := range s.Omitted {
		total += n
	}
	return total
}
