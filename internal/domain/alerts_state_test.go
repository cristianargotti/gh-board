package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestEvaluateRuleSelection(t *testing.T) {
	items := []domain.Item{
		newItem(1, "late", withIssueField(fxTarget, day(-3))),
		newItem(2, "blocked", withBlockedBy(1)),
	}
	cfg := alertsConfig()
	cfg.Alerts = domain.AlertRules{domain.AlertBlocked: {}}
	if got := evaluate(cfg, items...); !reflect.DeepEqual(got, []string{"blocked|warning|I_2|Bloqueada por 1 issue aberta"}) {
		t.Fatalf("only the listed rules run: %v", got)
	}
	cfg.Alerts = nil
	want := []string{"overdue|warning|I_1|Atrasada há 3 dias (alvo 05/10/2026)", "blocked|warning|I_2|Bloqueada por 1 issue aberta"}
	if got := evaluate(cfg, items...); !reflect.DeepEqual(got, want) {
		t.Fatalf("without an alerts section every rule runs: %v", got)
	}
	if got := evaluate(nil, newItem(3, "quiet")); len(got) != 0 {
		t.Fatalf("a quiet board has no alerts: %v", got)
	}
}

func TestEvaluateFirstSeen(t *testing.T) {
	earlier := fxNow.Add(-2 * time.Hour)
	previous := &domain.AlertState{Alerts: []domain.Alert{
		{RuleID: domain.AlertOverdue, Item: domain.ItemRef{NodeID: "I_1"}, FirstSeen: earlier},
		{RuleID: domain.AlertBlocked, Item: domain.ItemRef{NodeID: "I_2"}},
	}}
	items := []domain.Item{
		newItem(1, "late", withIssueField(fxTarget, day(-3))),
		newItem(2, "blocked", withBlockedBy(1)),
		newItem(3, "new", withBlockedBy(1)),
	}
	alerts := domain.EvaluateAlerts(domain.AlertInput{Config: alertsConfig(), Items: items, Now: fxNow, Previous: previous})
	if len(alerts) != 3 {
		t.Fatalf("alerts = %+v", alerts)
	}
	if !alerts[0].FirstSeen.Equal(earlier) {
		t.Fatalf("a known alert keeps its first seen instant, got %s", alerts[0].FirstSeen)
	}
	if !alerts[1].FirstSeen.Equal(fxNow) || !alerts[2].FirstSeen.Equal(fxNow) {
		t.Fatalf("new alerts and zero instants are stamped now: %s, %s", alerts[1].FirstSeen, alerts[2].FirstSeen)
	}
	fresh := domain.EvaluateAlerts(domain.AlertInput{Config: alertsConfig(), Items: items, Now: fxNow})
	if !fresh[0].FirstSeen.Equal(fxNow) {
		t.Fatalf("without a previous state every alert is first seen now: %s", fresh[0].FirstSeen)
	}
}

func TestItemRefOf(t *testing.T) {
	it := newItem(12, "  Title with "+string(rune(0x1b))+"[31mcolor"+string(rune(0x1b))+"[0m  ")
	ref := domain.ItemRefOf(it)
	want := domain.ItemRef{
		NodeID: "I_12", ProjectItemID: "PVTI_12", Repository: "acme/team-docs", Number: 12,
		Title: "Title with color", URL: "https://github.com/acme/team-docs/issues/12",
	}
	if ref != want {
		t.Fatalf("ItemRefOf = %+v, want %+v", ref, want)
	}
}

var isEpicCases = []struct {
	name string
	caps domain.Capabilities
	item domain.Item
	want bool
}{
	{"epic type", fixtureConfig().Capabilities, newItem(1, "e", withType(fxEpicType)), true},
	{"case insensitive", fixtureConfig().Capabilities, newItem(1, "e", withType("FEATURE")), true},
	{"task type", fixtureConfig().Capabilities, newItem(1, "t"), false},
	{"no type", fixtureConfig().Capabilities, domain.Item{}, false},
	{"no capability", domain.Capabilities{}, newItem(1, "e", withType(fxEpicType)), false},
	{"empty issue type", domain.Capabilities{Epic: &domain.EpicCapability{Field: fxEpic}}, newItem(1, "e", withType(fxEpicType)), false},
}

func TestIsEpic(t *testing.T) {
	for _, tc := range isEpicCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.IsEpic(tc.caps, tc.item); got != tc.want {
				t.Fatalf("IsEpic = %v, want %v", got, tc.want)
			}
		})
	}
}
