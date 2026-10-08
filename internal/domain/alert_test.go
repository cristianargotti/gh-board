package domain_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var alertState = domain.AlertState{
	GeneratedAt: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC),
	Project:     domain.ProjectRef{Owner: "acme", Number: 7},
	Summary:     "Sprint 5, 3 dias, 2 atrasadas, 1 bloqueada",
	Alerts: []domain.Alert{
		{RuleID: domain.AlertOverdue, Severity: domain.SeverityWarning, Item: domain.ItemRef{NodeID: "I_1"}, Message: "Atrasada"},
		{RuleID: domain.AlertOverdue, Severity: domain.SeverityWarning, Item: domain.ItemRef{NodeID: "I_2"}, Message: "Atrasada"},
		{RuleID: domain.AlertWIPExceeded, Severity: domain.SeverityCritical, Message: "WIP excedido"},
	},
}

func TestSeverityRank(t *testing.T) {
	if domain.SeverityCritical.Rank() <= domain.SeverityWarning.Rank() || domain.SeverityWarning.Rank() <= domain.SeverityInfo.Rank() {
		t.Fatal("severity ranks are not ordered")
	}
	if domain.Severity("odd").Rank() != 0 {
		t.Fatal("unknown severity must rank zero")
	}
}

func TestAlertState(t *testing.T) {
	if fp := alertState.Alerts[0].Fingerprint(); fp != "overdue|I_1" {
		t.Fatalf("Fingerprint = %q", fp)
	}
	if fp := alertState.Alerts[2].Fingerprint(); fp != "wip_exceeded|" {
		t.Fatalf("board-level Fingerprint = %q", fp)
	}
	found, ok := alertState.Find("overdue|I_2")
	if !ok || found.Item.NodeID != "I_2" {
		t.Fatalf("Find = %+v, %v", found, ok)
	}
	if _, ok := alertState.Find("blocked|I_2"); ok {
		t.Fatal("missing alert must not be found")
	}
	counts := alertState.CountByRule()
	if counts[domain.AlertOverdue] != 2 || counts[domain.AlertWIPExceeded] != 1 || len(counts) != 2 {
		t.Fatalf("CountByRule = %v", counts)
	}
}
