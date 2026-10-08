package domain

import (
	"fmt"
	"strings"
	"time"
)

// PT-BR words the messages repeat.
const (
	dayOne  = "dia"
	dayMany = "dias"
)

// plural picks the singular or plural form for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// CurrentSprint finds the iteration in progress on the configured sprint
// field of the discovered project, on the board-zone date of now.
func CurrentSprint(cfg *Config, project Project, now time.Time) (Field, Iteration, bool) {
	caps := CapabilitiesOf(cfg)
	if caps.Sprint == nil || caps.Sprint.Field == "" {
		return Field{}, Iteration{}, false
	}
	field, ok := project.FieldByName(caps.Sprint.Field)
	if !ok {
		return Field{}, Iteration{}, false
	}
	sprint, ok := field.IterationAt(Today(now, LocationOf(cfg)))
	return field, sprint, ok
}

func (ev *evaluator) wipExceeded() []Alert {
	policy := PolicyOf(ev.cfg)
	if len(policy.WIP) == 0 {
		return nil
	}
	excess := policy.Exceeded(CountByStatus(ev.items, StatusFieldName(ev.cfg)))
	if len(excess) == 0 {
		return nil
	}
	parts := make([]string, 0, len(excess))
	for _, e := range excess {
		parts = append(parts, fmt.Sprintf("%s %d/%d", e.Status, e.Count, e.Limit))
	}
	return []Alert{{RuleID: AlertWIPExceeded, Severity: SeverityCritical, Message: "WIP excedido: " + strings.Join(parts, ", ")}}
}

func (ev *evaluator) sprintEnding() []Alert {
	field, sprint, ok := CurrentSprint(ev.cfg, ev.in.Project, ev.now)
	if !ok {
		return nil
	}
	left := sprint.DaysLeft(Today(ev.now, ev.loc))
	if left > ev.days(AlertSprintEnding, DefaultSprintEndingDays) {
		return nil
	}
	open := 0
	for _, it := range ev.items {
		if it.Text(field.Name) == sprint.Title {
			open++
		}
	}
	msg := fmt.Sprintf("%s %s com %d %s", sprint.Title, sprintEndsIn(left), open, plural(open, "item aberto", "itens abertos"))
	return []Alert{{RuleID: AlertSprintEnding, Severity: SeverityInfo, Message: msg}}
}

// sprintEndsIn words the days left of a sprint; the last day reads as
// today rather than as zero days.
func sprintEndsIn(left int) string {
	if left == 0 {
		return "termina hoje"
	}
	return fmt.Sprintf("termina em %d %s", left, plural(left, dayOne, dayMany))
}

// summaryRules are the rules the status line counts, with their PT-BR
// forms; sprint_ending is implied by the days left.
var summaryRules = []struct{ rule, one, many string }{
	{AlertOverdue, "atrasada", "atrasadas"},
	{AlertBlocked, "bloqueada", "bloqueadas"},
	{AlertTriageSLA, "entrada fora do SLA", "entradas fora do SLA"},
	{AlertWIPExceeded, "WIP excedido", "WIP excedido"},
	{AlertStaleActive, "parada", "paradas"},
	{AlertEpicWithoutDates, "épico sem datas", "épicos sem datas"},
}

// AlertSummary writes the one-line summary of alerts.json, the status line
// of the mod: sprint, days left and the counts per rule, in PT-BR.
func AlertSummary(sprint *SprintSummary, alerts []Alert) string {
	var parts []string
	if sprint != nil {
		parts = append(parts, sprint.Title, fmt.Sprintf("%d %s", sprint.DaysLeft, plural(sprint.DaysLeft, dayOne, dayMany)))
	}
	counts := AlertState{Alerts: alerts}.CountByRule()
	for _, s := range summaryRules {
		if n := counts[s.rule]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, plural(n, s.one, s.many)))
		}
	}
	if len(parts) == 0 {
		return "sem alertas"
	}
	return strings.Join(parts, " · ")
}
