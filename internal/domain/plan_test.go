package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var planCreated = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func samplePlan() domain.Plan {
	return domain.Plan{
		ID: "p1", KitVersion: "0.1.0", CreatedAt: planCreated, ExpiresAt: planCreated.Add(domain.PlanExpiry),
		Actor: "ana", Host: "github.com", Project: domain.ProjectRef{Owner: "acme", Number: 7},
		Command: "close #12", Description: "Fecha a tarefa #12",
		Steps: []domain.Step{
			{Index: 0, Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: "I_1", Number: 12}, Field: "Status", Before: "IN PROGRESS", After: "DONE"},
			{Index: 1, Operation: domain.OpCloseIssue, Target: domain.Target{NodeID: "I_1", Number: 12}, Before: "OPEN", After: "CLOSED"},
			{Index: 2, Operation: domain.OpAddComment, Target: domain.Target{NodeID: "I_2", Number: 13}, After: "ok"},
		},
	}
}

func TestPlanHash(t *testing.T) {
	p := samplePlan()
	first, err := p.ComputeHash()
	if err != nil || len(first) != 64 {
		t.Fatalf("ComputeHash = %q, %v", first, err)
	}
	p.Hash = first
	again, _ := p.ComputeHash()
	if again != first {
		t.Fatal("the stored hash must not influence the hash")
	}
	p.Steps[1].After = "OPEN"
	changed, _ := p.ComputeHash()
	if changed == first {
		t.Fatal("a changed step must change the hash")
	}
}

func TestPlanExpiryAndTargets(t *testing.T) {
	p := samplePlan()
	if p.Expired(planCreated.Add(29 * time.Minute)) {
		t.Fatal("plan expired too early")
	}
	if !p.Expired(planCreated.Add(domain.PlanExpiry)) {
		t.Fatal("plan must expire at ExpiresAt")
	}
	targets := p.Targets()
	if len(targets) != 2 || targets[0] != "I_1" || targets[1] != "I_2" {
		t.Fatalf("Targets = %v", targets)
	}
	empty := domain.Plan{Steps: []domain.Step{{Target: domain.Target{}}}}
	if len(empty.Targets()) != 0 {
		t.Fatal("steps without node id are not targets")
	}
}

func TestPlanItemsAndTargetRefs(t *testing.T) {
	p := domain.Plan{Steps: []domain.Step{
		{Target: domain.Target{NodeID: "I_1", Repository: "acme/app", Number: 7, Title: "Issue"}},
		{Target: domain.Target{NodeID: "I_1", Repository: "acme/app", Number: 7}},
		{Target: domain.Target{NodeID: "R_1", Repository: "acme/app"}},
		{Target: domain.Target{NodeID: "PVT_1", Title: "New\x1b[31m board"}},
		{Target: domain.Target{}},
	}}
	got := strings.Join(p.Items(), ",")
	if got != "acme/app#7,acme/app,New board" {
		t.Fatalf("Items = %q", got)
	}
	if (domain.Target{}).Ref() != "" {
		t.Fatal("an empty target has no reference")
	}
}
