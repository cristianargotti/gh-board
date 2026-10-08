package domain_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var statusCap = &domain.StatusCapability{
	Field: "Status", Backlog: []string{"BACKLOG"}, Ready: []string{"READY TO DEV"},
	Active: []string{"IN PROGRESS", "TEST / VALIDATION"}, Done: []string{"DONE"},
}

var classifyCases = map[string]domain.StatusClass{
	"BACKLOG": domain.StatusBacklog, "READY TO DEV": domain.StatusReady, "IN PROGRESS": domain.StatusActive,
	"TEST / VALIDATION": domain.StatusActive, "DONE": domain.StatusDone, "ARCHIVED": domain.StatusUnknown,
}

func TestStatusClassify(t *testing.T) {
	for option, want := range classifyCases {
		if got := statusCap.Classify(option); got != want {
			t.Errorf("Classify(%q) = %v, want %v", option, got, want)
		}
	}
	if !statusCap.IsDone("DONE") || statusCap.IsDone("BACKLOG") {
		t.Fatal("IsDone is wrong")
	}
	var unmapped *domain.StatusCapability
	if unmapped.Classify("DONE") != domain.StatusUnknown || unmapped.IsDone("DONE") {
		t.Fatal("a nil capability maps nothing")
	}
}

func TestKnownIdentifiers(t *testing.T) {
	if len(domain.AlertRuleIDs) != 7 {
		t.Fatalf("AlertRuleIDs = %v", domain.AlertRuleIDs)
	}
	if !domain.Contains(domain.AlertRuleIDs, domain.AlertStaleActive) || domain.Contains(domain.AlertRuleIDs, "nope") {
		t.Fatal("Contains is wrong")
	}
	if !domain.Contains(domain.DigestMetrics, "autonomy") || !domain.Contains(domain.TidyInheritable, "epic") {
		t.Fatal("known identifiers are incomplete")
	}
}
