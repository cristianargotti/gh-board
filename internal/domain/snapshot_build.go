package domain

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"
)

// SnapshotInput is what context needs to assemble a snapshot. For names
// the login whose open items fill Mine and defaults to Viewer; Alerts may
// carry a previous evaluation and are evaluated here when nil; Budget in
// tokens defaults to DefaultContextBudget; Measure is how the caller
// counts the tokens of the output it emits (FitBudgetWith), nil for the
// JSON estimate.
type SnapshotInput struct {
	Config  *Config
	Project Project
	Items   []Item
	Viewer  string
	For     string
	Now     time.Time
	Alerts  []Alert
	Budget  int
	Measure func(Snapshot) int
}

// BuildSnapshot assembles the sections of section 7 in their order and
// fits them to the token budget, counting what it omits.
func BuildSnapshot(in SnapshotInput) Snapshot {
	who := in.For
	if who == "" {
		who = in.Viewer
	}
	alerts := in.Alerts
	if alerts == nil {
		alerts = EvaluateAlerts(AlertInput{Config: in.Config, Project: in.Project, Items: in.Items, Now: in.Now})
	}
	mine := make([]Item, 0)
	for _, it := range Pending(in.Config, in.Items) {
		if it.AssignedTo(who) {
			mine = append(mine, it)
		}
	}
	week := WeekStart(in.Now, LocationOf(in.Config))
	s := Snapshot{
		GeneratedAt: in.Now, Project: in.Project.Ref, Title: CleanTitle(in.Project.Title), Viewer: in.Viewer,
		Rules:      TeamRulesOf(in.Config),
		Sprint:     SprintSummaryOf(in.Config, in.Project, in.Items, in.Now),
		Mine:       SummarizeItems(in.Config, SortByUrgency(in.Config, mine)),
		Attention:  SortBySeverity(alerts),
		Triage:     SummarizeItems(in.Config, TriageQueue(in.Config, in.Items)),
		Epics:      EpicsProgress(in.Config, in.Items),
		Deliveries: SummarizeItems(in.Config, DeliveredSince(in.Items, week)),
		Omitted:    map[string]int{}, Completeness: Complete,
	}
	FitBudgetWith(&s, in.Budget, in.Measure)
	return s
}

// TeamRulesOf compacts the team process for a prompt, in PT-BR with the
// status names as the board spells them. Generic mode has no rules.
func TeamRulesOf(cfg *Config) TeamRules {
	if cfg == nil {
		return TeamRules{}
	}
	rules := TeamRules{Language: cfg.Language}
	for _, from := range slices.Sorted(maps.Keys(cfg.Policy.Transitions)) {
		rules.Flow = append(rules.Flow, from+" -> "+strings.Join(cfg.Policy.Transitions[from], " | "))
	}
	for _, status := range slices.Sorted(maps.Keys(cfg.Policy.WIP)) {
		rules.WIP = append(rules.WIP, fmt.Sprintf("%s: máximo %d", status, cfg.Policy.WIP[status]))
	}
	rules.SLA = slaRules(cfg)
	for _, r := range cfg.Rituals {
		rules.Rituals = append(rules.Rituals, fmt.Sprintf("%s: %s (%s)", r.Name, r.When, strings.Join(r.Reads, ", ")))
	}
	return rules
}

func slaRules(cfg *Config) []string {
	var out []string
	caps := cfg.Capabilities
	if sla, ok := caps.Triage.SLA(); ok {
		line := fmt.Sprintf("Triagem: decisão em %d dias úteis", sla)
		if caps.Triage.UrgentLabel != "" {
			line += fmt.Sprintf("; %s: no mesmo dia", caps.Triage.UrgentLabel)
		}
		out = append(out, line)
	}
	if task := caps.Task; task != nil && task.MaxEstimateDays > 0 {
		out = append(out, fmt.Sprintf("Tarefa: estimativa máxima de %d dias", task.MaxEstimateDays))
	}
	if cfg.Policy.BulkThreshold > 0 {
		out = append(out, fmt.Sprintf("Escrita em lote acima de %d itens exige plano", cfg.Policy.BulkThreshold))
	}
	return out
}

// SprintSummaryOf describes the current iteration with its open and done
// items, or nil when no sprint is in progress.
func SprintSummaryOf(cfg *Config, project Project, items []Item, now time.Time) *SprintSummary {
	field, sprint, ok := CurrentSprint(cfg, project, now)
	if !ok {
		return nil
	}
	sum := IterationSummary(sprint, Today(now, LocationOf(cfg)))
	for _, it := range items {
		if it.Archived || it.Text(field.Name) != sprint.Title {
			continue
		}
		if IsDone(cfg, it) {
			sum.Done++
		} else {
			sum.Open++
		}
	}
	return sum
}

// IterationSummary describes an iteration on a calendar date, without
// item counts.
func IterationSummary(it Iteration, day time.Time) *SprintSummary {
	return &SprintSummary{Title: it.Title, Start: DateText(it.Start), End: DateText(it.LastDay()), DaysLeft: it.DaysLeft(day)}
}

// SortByUrgency orders items most urgent first: the items with a target
// date by that date, so the overdue ones lead, then the rest by their
// last status change, the most recently moved first. The order is stable.
func SortByUrgency(cfg *Config, items []Item) []Item {
	dates, status := CapabilitiesOf(cfg).Dates, StatusFieldName(cfg)
	out := append([]Item(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		ti, oki := ItemDate(out[i], dates, DateTarget, time.UTC)
		tj, okj := ItemDate(out[j], dates, DateTarget, time.UTC)
		switch {
		case oki && okj:
			return ti.Before(tj)
		case oki != okj:
			return oki
		}
		return out[i].Values[status].UpdatedAt.After(out[j].Values[status].UpdatedAt)
	})
	return out
}

// SortBySeverity orders alerts most severe first, keeping the rule and
// item order inside each severity.
func SortBySeverity(alerts []Alert) []Alert {
	if alerts == nil {
		return nil
	}
	out := append([]Alert(nil), alerts...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severity.Rank() > out[j].Severity.Rank() })
	return out
}

// TriageQueue lists the undecided entries, oldest first.
func TriageQueue(cfg *Config, items []Item) []Item {
	tri := CapabilitiesOf(cfg).Triage
	out := make([]Item, 0)
	for _, it := range Pending(cfg, items) {
		if tri.Undecided(it) {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Issue.CreatedAt.Before(out[j].Issue.CreatedAt) })
	return out
}

// EpicsProgress lists the open epics with sub-issue progress and dates.
func EpicsProgress(cfg *Config, items []Item) []EpicProgress {
	caps := CapabilitiesOf(cfg)
	out := make([]EpicProgress, 0)
	for _, it := range Pending(cfg, items) {
		if !IsEpic(caps, it) {
			continue
		}
		status, _ := ItemStatus(cfg, it)
		out = append(out, EpicProgress{
			Ref: it.Issue.Ref(), Title: CleanTitle(it.Issue.Title), Status: status, Owner: firstLogin(it.Issue.Assignees),
			Total: it.Issue.SubIssues.Total, Completed: it.Issue.SubIssues.Completed,
			Start: ItemDateText(it, caps.Dates, DateStart), Target: ItemDateText(it, caps.Dates, DateTarget),
		})
	}
	return out
}

// DeliveredSince lists the items closed at or after the instant.
func DeliveredSince(items []Item, since time.Time) []Item {
	out := make([]Item, 0)
	for _, it := range items {
		if it.Issue.ClosedAt != nil && !it.Issue.ClosedAt.Before(since) {
			out = append(out, it)
		}
	}
	return out
}

// SummarizeItem reduces an item to what a prompt needs, with the title
// already cleaned and truncated.
func SummarizeItem(cfg *Config, it Item) ItemSummary {
	caps := CapabilitiesOf(cfg)
	status, _ := ItemStatus(cfg, it)
	s := ItemSummary{
		Ref: it.Issue.Ref(), NodeID: it.Issue.NodeID, Title: CleanTitle(it.Issue.Title), Status: status,
		Assignees: logins(it.Issue.Assignees), URL: it.Issue.URL,
		Target: ItemDateText(it, caps.Dates, DateTarget),
	}
	if caps.Lane != nil {
		s.Lane = it.Text(caps.Lane.Field)
	}
	if caps.Epic != nil {
		s.Epic = it.Text(caps.Epic.Field)
	}
	if caps.Sprint != nil {
		s.Sprint = it.Text(caps.Sprint.Field)
	}
	return s
}

// SummarizeItems maps SummarizeItem over the items, never returning nil.
func SummarizeItems(cfg *Config, items []Item) []ItemSummary {
	out := make([]ItemSummary, 0, len(items))
	for _, it := range items {
		out = append(out, SummarizeItem(cfg, it))
	}
	return out
}

func logins(users []User) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.Login)
	}
	return out
}

func firstLogin(users []User) string {
	if len(users) == 0 {
		return ""
	}
	return users[0].Login
}
