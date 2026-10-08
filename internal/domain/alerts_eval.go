package domain

import (
	"fmt"
	"strings"
	"time"
)

// Defaults of the alert rules when board.yml enables them without options.
// The values are the ones the reference board.yml of section 5.2 declares.
const (
	DefaultSprintEndingDays = 2
	DefaultStaleActiveDays  = 5
	DefaultTriageSLADays    = 2
	// OverdueCriticalDays is the delay that turns an overdue warning
	// critical, which gives the rule an escalation watch can notify.
	OverdueCriticalDays = 7
)

// AlertInput is everything the evaluator reads. Previous keeps FirstSeen
// of the alerts already known.
type AlertInput struct {
	Config   *Config
	Project  Project
	Items    []Item
	Now      time.Time
	Previous *AlertState
}

type evaluator struct {
	cfg   *Config
	caps  Capabilities
	loc   *time.Location
	now   time.Time
	in    AlertInput
	items []Item
}

func newEvaluator(in AlertInput) *evaluator {
	return &evaluator{
		cfg: in.Config, caps: CapabilitiesOf(in.Config), loc: LocationOf(in.Config),
		now: in.Now, in: in, items: Pending(in.Config, in.Items),
	}
}

// EvaluateAlerts is the one alert function of section 9: attention,
// context, watch and digest all read it. Alerts come out in rule order,
// then item order, with stable ids and PT-BR messages. A rule whose
// capability is not declared finds nothing.
func EvaluateAlerts(in AlertInput) []Alert {
	ev := newEvaluator(in)
	rules := map[string]func() []Alert{
		AlertOverdue: ev.overdue, AlertBlocked: ev.blocked, AlertTriageSLA: ev.triageSLA,
		AlertWIPExceeded: ev.wipExceeded, AlertSprintEnding: ev.sprintEnding,
		AlertStaleActive: ev.staleActive, AlertEpicWithoutDates: ev.epicWithoutDates,
	}
	out := []Alert{}
	for _, id := range AlertRuleIDs {
		if ev.enabled(id) {
			out = append(out, rules[id]()...)
		}
	}
	return ev.stamp(out)
}

// enabled: without an alerts section every rule runs; with one, only the
// listed rules run.
func (ev *evaluator) enabled(rule string) bool {
	if ev.cfg == nil || ev.cfg.Alerts == nil {
		return true
	}
	_, ok := ev.cfg.Alerts[rule]
	return ok
}

func (ev *evaluator) days(rule string, fallback int) int {
	if ev.cfg != nil {
		if r, ok := ev.cfg.Alerts[rule]; ok && r.Days > 0 {
			return r.Days
		}
	}
	return fallback
}

func (ev *evaluator) stamp(alerts []Alert) []Alert {
	for i := range alerts {
		alerts[i].FirstSeen = ev.now
		if ev.in.Previous == nil {
			continue
		}
		if prev, ok := ev.in.Previous.Find(alerts[i].Fingerprint()); ok && !prev.FirstSeen.IsZero() {
			alerts[i].FirstSeen = prev.FirstSeen
		}
	}
	return alerts
}

func (ev *evaluator) alert(rule string, sev Severity, it Item, msg string) Alert {
	return Alert{RuleID: rule, Severity: sev, Item: ItemRefOf(it), Message: msg}
}

// ItemRefOf reduces an item to the identity an alert carries.
func ItemRefOf(it Item) ItemRef {
	return ItemRef{
		NodeID: it.Issue.NodeID, ProjectItemID: it.ProjectItemID,
		Repository: it.Issue.Owner + "/" + it.Issue.Repo, Number: it.Issue.Number,
		Title: CleanTitle(it.Issue.Title), URL: it.Issue.URL,
	}
}

func (ev *evaluator) overdue() []Alert {
	var out []Alert
	for _, it := range ev.items {
		target, ok := ItemDate(it, ev.caps.Dates, DateTarget, ev.loc)
		if !ok {
			continue
		}
		late := -DaysFromToday(target, ev.now, ev.loc)
		if late <= 0 {
			continue
		}
		sev := SeverityWarning
		if late >= OverdueCriticalDays {
			sev = SeverityCritical
		}
		msg := fmt.Sprintf("Atrasada há %d %s (alvo %s)", late, plural(late, dayOne, dayMany), DayText(target))
		out = append(out, ev.alert(AlertOverdue, sev, it, msg))
	}
	return out
}

func (ev *evaluator) blocked() []Alert {
	var out []Alert
	for _, it := range ev.items {
		msg := ""
		if ev.caps.Blocked != nil {
			if v := it.Text(ev.caps.Blocked.Field); v != "" {
				msg = fmt.Sprintf("Bloqueada: %s = %s", ev.caps.Blocked.Field, CleanTitle(v))
			}
		}
		if n := it.Issue.BlockedByCount; msg == "" && n > 0 {
			msg = fmt.Sprintf("Bloqueada por %d %s", n, plural(n, "issue aberta", "issues abertas"))
		}
		if msg != "" {
			out = append(out, ev.alert(AlertBlocked, SeverityWarning, it, msg))
		}
	}
	return out
}

// SLA returns the business days of the triage SLA: the declared value, or
// the reference default when the capability declares none. It is false
// only when the capability is absent.
func (t *TriageCapability) SLA() (int, bool) {
	if t == nil {
		return 0, false
	}
	if t.SLABusinessDays <= 0 {
		return DefaultTriageSLADays, true
	}
	return t.SLABusinessDays, true
}

// Undecided reports whether the item is a triage entry still waiting for
// a decision: it carries the triage label and the decision field is empty.
func (t *TriageCapability) Undecided(it Item) bool {
	if t == nil || t.Label == "" || !it.HasLabel(t.Label) {
		return false
	}
	return t.DecisionField == "" || it.Text(t.DecisionField) == ""
}

func (ev *evaluator) triageSLA() []Alert {
	var out []Alert
	for _, it := range ev.items {
		if !ev.caps.Triage.Undecided(it) {
			continue
		}
		if a, ok := ev.triageAlert(it); ok {
			out = append(out, a)
		}
	}
	return out
}

// triageAlert applies the SLA to one undecided entry: urgent entries are
// due the same day, the others after the business days of the SLA.
func (ev *evaluator) triageAlert(it Item) (Alert, bool) {
	tri := ev.caps.Triage
	created := it.Issue.CreatedAt
	if tri.UrgentLabel != "" && it.HasLabel(tri.UrgentLabel) {
		if DaysBetween(created, ev.now, ev.loc) <= 0 {
			return Alert{}, false
		}
		msg := fmt.Sprintf("Entrada urgente sem decisão desde %s (SLA: mesmo dia)", FormatDay(created, ev.loc))
		return ev.alert(AlertTriageSLA, SeverityCritical, it, msg), true
	}
	sla, ok := tri.SLA()
	if !ok {
		return Alert{}, false
	}
	elapsed := BusinessDaysBetween(created, ev.now, ev.loc)
	if elapsed <= sla {
		return Alert{}, false
	}
	msg := fmt.Sprintf("Entrada sem decisão há %d dias úteis (SLA: %d)", elapsed, sla)
	return ev.alert(AlertTriageSLA, SeverityWarning, it, msg), true
}

func (ev *evaluator) staleActive() []Alert {
	if ev.caps.Status == nil {
		return nil
	}
	limit := ev.days(AlertStaleActive, DefaultStaleActiveDays)
	var out []Alert
	for _, it := range ev.items {
		v, ok := it.Value(ev.caps.Status.Field)
		if !ok || v.UpdatedAt.IsZero() || ev.caps.Status.Classify(v.Value) != StatusActive {
			continue
		}
		idle := DaysBetween(v.UpdatedAt, ev.now, ev.loc)
		if idle <= limit {
			continue
		}
		msg := fmt.Sprintf("Sem movimento há %d %s em %s", idle, plural(idle, dayOne, dayMany), v.Value)
		out = append(out, ev.alert(AlertStaleActive, SeverityWarning, it, msg))
	}
	return out
}

func (ev *evaluator) epicWithoutDates() []Alert {
	if ev.caps.Epic == nil || ev.caps.Dates == nil {
		return nil
	}
	var out []Alert
	for _, it := range ev.items {
		if !IsEpic(ev.caps, it) {
			continue
		}
		_, hasStart := ItemDate(it, ev.caps.Dates, DateStart, ev.loc)
		_, hasTarget := ItemDate(it, ev.caps.Dates, DateTarget, ev.loc)
		if msg := missingDatesMessage(hasStart, hasTarget); msg != "" {
			out = append(out, ev.alert(AlertEpicWithoutDates, SeverityInfo, it, msg))
		}
	}
	return out
}

func missingDatesMessage(hasStart, hasTarget bool) string {
	switch {
	case !hasStart && !hasTarget:
		return "Épico sem data de início e sem data alvo"
	case !hasStart:
		return "Épico sem data de início"
	case !hasTarget:
		return "Épico sem data alvo"
	default:
		return ""
	}
}

// IsEpic reports whether the item carries the epic issue type.
func IsEpic(caps Capabilities, it Item) bool {
	return caps.Epic != nil && caps.Epic.IssueType != "" && it.Issue.Type != nil &&
		strings.EqualFold(it.Issue.Type.Name, caps.Epic.IssueType)
}
