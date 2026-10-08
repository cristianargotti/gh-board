package domain_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// alertsConfig raises the WIP limits so that per-rule cases never trip
// the board-level rule by accident.
func alertsConfig() *domain.Config {
	cfg := fixtureConfig()
	cfg.Policy.WIP = map[string]int{stActive: 6, stTest: 5}
	return cfg
}

func evaluate(cfg *domain.Config, items ...domain.Item) []string {
	alerts := domain.EvaluateAlerts(domain.AlertInput{Config: cfg, Project: fixtureProject(), Items: items, Now: fxNow})
	out := make([]string, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, fmt.Sprintf("%s|%s|%s|%s", a.RuleID, a.Severity, a.Item.NodeID, a.Message))
	}
	return out
}

func at(offsetDays int, hour int) time.Time {
	return time.Date(2026, 10, 8+offsetDays, hour, 0, 0, 0, fxLoc)
}

var overdueCases = []struct {
	name  string
	items []domain.Item
	want  []string
}{
	{"three days late", []domain.Item{newItem(1, "a", withIssueField(fxTarget, day(-3)))}, []string{"overdue|warning|I_1|Atrasada há 3 dias (alvo 05/10/2026)"}},
	{"one day late", []domain.Item{newItem(1, "a", withIssueField(fxTarget, day(-1)))}, []string{"overdue|warning|I_1|Atrasada há 1 dia (alvo 07/10/2026)"}},
	{"seven days late is critical", []domain.Item{newItem(2, "b", withIssueField(fxTarget, day(-7)))}, []string{"overdue|critical|I_2|Atrasada há 7 dias (alvo 01/10/2026)"}},
	{"due today", []domain.Item{newItem(3, "c", withIssueField(fxTarget, day(0)))}, []string{}},
	{"due tomorrow", []domain.Item{newItem(4, "d", withIssueField(fxTarget, day(1)))}, []string{}},
	{"finished items are never overdue", []domain.Item{
		newItem(5, "closed", withIssueField(fxTarget, day(-3)), withClosed(fxNow)),
		newItem(6, "archived", withIssueField(fxTarget, day(-3)), archived()),
		newItem(7, "done status", withIssueField(fxTarget, day(-3)), withStatus(stDone, fxNow)),
	}, []string{}},
	{"item order is kept", []domain.Item{
		newItem(9, "late", withIssueField(fxTarget, day(-2))), newItem(8, "later", withIssueField(fxTarget, day(-9))),
	}, []string{"overdue|warning|I_9|Atrasada há 2 dias (alvo 06/10/2026)", "overdue|critical|I_8|Atrasada há 9 dias (alvo 29/09/2026)"}},
}

func TestEvaluateOverdue(t *testing.T) {
	for _, tc := range overdueCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(alertsConfig(), tc.items...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("alerts = %v, want %v", got, tc.want)
			}
		})
	}
}

var blockedCases = []struct {
	name  string
	cfg   *domain.Config
	items []domain.Item
	want  []string
}{
	{"blocked field", alertsConfig(), []domain.Item{newItem(5, "a", withField(fxBlocked, "Bloqueada"))}, []string{"blocked|warning|I_5|Bloqueada: Situação = Bloqueada"}},
	{"blocked by issues", alertsConfig(), []domain.Item{newItem(6, "a", withBlockedBy(2))}, []string{"blocked|warning|I_6|Bloqueada por 2 issues abertas"}},
	{"blocked by one issue", alertsConfig(), []domain.Item{newItem(7, "a", withBlockedBy(1))}, []string{"blocked|warning|I_7|Bloqueada por 1 issue aberta"}},
	{"field wins over count", alertsConfig(), []domain.Item{newItem(8, "a", withField(fxBlocked, "Bloqueada"), withBlockedBy(3))}, []string{"blocked|warning|I_8|Bloqueada: Situação = Bloqueada"}},
	{"generic mode reads blockedBy", nil, []domain.Item{newItem(9, "a", withBlockedBy(1), withField(fxBlocked, "Bloqueada"))}, []string{"blocked|warning|I_9|Bloqueada por 1 issue aberta"}},
	{"nothing blocked", alertsConfig(), []domain.Item{newItem(10, "a")}, []string{}},
}

func TestEvaluateBlocked(t *testing.T) {
	for _, tc := range blockedCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(tc.cfg, tc.items...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("alerts = %v, want %v", got, tc.want)
			}
		})
	}
}

var triageCases = []struct {
	name  string
	items []domain.Item
	want  []string
}{
	{"past the SLA", []domain.Item{newItem(7, "a", withLabel(fxTriage), withCreated(at(-3, 10)))}, []string{"triage_sla|warning|I_7|Entrada sem decisão há 3 dias úteis (SLA: 2)"}},
	{"within the SLA", []domain.Item{newItem(8, "a", withLabel(fxTriage), withCreated(at(-2, 10)))}, []string{}},
	{"weekend does not count", []domain.Item{newItem(1, "a", withLabel(fxTriage), withCreated(at(-7, 10)))}, []string{"triage_sla|warning|I_1|Entrada sem decisão há 5 dias úteis (SLA: 2)"}},
	{"urgent entry from yesterday", []domain.Item{newItem(9, "a", withLabel(fxTriage, fxUrgent), withCreated(at(-1, 18)))}, []string{"triage_sla|critical|I_9|Entrada urgente sem decisão desde 07/10/2026 (SLA: mesmo dia)"}},
	{"urgent entry from today", []domain.Item{newItem(10, "a", withLabel(fxTriage, fxUrgent), withCreated(at(0, 9)))}, []string{}},
	{"decided entry", []domain.Item{newItem(11, "a", withLabel(fxTriage), withCreated(at(-7, 10)), withField(fxDecision, "Ensinar"))}, []string{}},
	{"urgent label alone is not an entry", []domain.Item{newItem(12, "a", withLabel(fxUrgent), withCreated(at(-7, 10)))}, []string{}},
}

func TestEvaluateTriageSLA(t *testing.T) {
	for _, tc := range triageCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(alertsConfig(), tc.items...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("alerts = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTriageSLADefaults(t *testing.T) {
	cfg := alertsConfig()
	cfg.Capabilities.Triage.SLABusinessDays = 0
	if sla, ok := cfg.Capabilities.Triage.SLA(); !ok || sla != domain.DefaultTriageSLADays {
		t.Fatalf("SLA = %d, %v", sla, ok)
	}
	var absent *domain.TriageCapability
	if _, ok := absent.SLA(); ok || absent.Undecided(newItem(1, "a", withLabel(fxTriage))) {
		t.Fatal("an absent capability has no SLA and no entries")
	}
	entry := newItem(7, "a", withLabel(fxTriage), withCreated(at(-3, 10)))
	want := []string{"triage_sla|warning|I_7|Entrada sem decisão há 3 dias úteis (SLA: 2)"}
	if got := evaluate(cfg, entry); !reflect.DeepEqual(got, want) {
		t.Fatalf("alerts = %v, want %v", got, want)
	}
	cfg.Capabilities.Triage = nil
	if got := evaluate(cfg, entry); len(got) != 0 {
		t.Fatalf("without the capability there is no triage rule: %v", got)
	}
}

var staleCases = []struct {
	name  string
	items []domain.Item
	want  []string
}{
	{"six idle days", []domain.Item{newItem(12, "a", withStatus(stActive, at(-6, 12)))}, []string{"stale_active|warning|I_12|Sem movimento há 6 dias em IN PROGRESS"}},
	{"five idle days is the limit", []domain.Item{newItem(13, "a", withStatus(stActive, at(-5, 12)))}, []string{}},
	{"no timestamp", []domain.Item{newItem(14, "a", withStatus(stActive, time.Time{}))}, []string{}},
	{"ready is not active", []domain.Item{newItem(15, "a", withStatus(stReady, at(-10, 12)))}, []string{}},
	{"validation is active", []domain.Item{newItem(16, "a", withStatus(stTest, at(-8, 12)))}, []string{"stale_active|warning|I_16|Sem movimento há 8 dias em TEST / VALIDATION"}},
}

func TestEvaluateStaleActive(t *testing.T) {
	for _, tc := range staleCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(alertsConfig(), tc.items...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("alerts = %v, want %v", got, tc.want)
			}
		})
	}
	cfg := alertsConfig()
	cfg.Alerts[domain.AlertStaleActive] = domain.AlertRule{Days: 1}
	want := []string{"stale_active|warning|I_1|Sem movimento há 2 dias em IN PROGRESS"}
	if got := evaluate(cfg, newItem(1, "a", withStatus(stActive, at(-2, 12)))); !reflect.DeepEqual(got, want) {
		t.Fatalf("days option: alerts = %v, want %v", got, want)
	}
	if got := evaluate(nil, newItem(1, "a", withStatus(stActive, at(-20, 12)))); len(got) != 0 {
		t.Fatalf("generic mode has no active class: %v", got)
	}
}

var epicDateCases = []struct {
	name  string
	items []domain.Item
	want  []string
}{
	{"no dates", []domain.Item{newItem(16, "e", withType(fxEpicType))}, []string{"epic_without_dates|info|I_16|Épico sem data de início e sem data alvo"}},
	{"start only", []domain.Item{newItem(17, "e", withType(fxEpicType), withIssueField(fxStart, day(-1)))}, []string{"epic_without_dates|info|I_17|Épico sem data alvo"}},
	{"target only", []domain.Item{newItem(18, "e", withType("feature"), withIssueField(fxTarget, day(10)))}, []string{"epic_without_dates|info|I_18|Épico sem data de início"}},
	{"both dates", []domain.Item{newItem(19, "e", withType(fxEpicType), withIssueField(fxStart, day(-1)), withIssueField(fxTarget, day(10)))}, []string{}},
	{"tasks are not epics", []domain.Item{newItem(20, "t")}, []string{}},
}

func TestEvaluateEpicWithoutDates(t *testing.T) {
	for _, tc := range epicDateCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(alertsConfig(), tc.items...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("alerts = %v, want %v", got, tc.want)
			}
		})
	}
}
