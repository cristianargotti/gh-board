package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func unsealedPlan() domain.Plan {
	return domain.Plan{
		ID: "p1", KitVersion: "0.1.0", CreatedAt: fxNow, Actor: fxViewer, Host: "github.com",
		Project: domain.ProjectRef{Owner: fxOwner, Number: 2}, Command: "close #12", Description: "Fecha a tarefa #12",
		Steps: []domain.Step{
			{Index: 7, Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: "I_1", Number: 12}, Field: fxStatus, Before: stActive, After: stDone},
			{Index: 7, Operation: domain.OpCloseIssue, Target: domain.Target{NodeID: "I_1", Number: 12}, Before: "OPEN", After: "CLOSED"},
		},
	}
}

func TestNewPlanID(t *testing.T) {
	id := domain.NewPlanID(fxNow, "alice|close #12")
	if !strings.HasPrefix(id, "20261008T150000Z-") || len(id) != len("20261008T150000Z-")+8 {
		t.Fatalf("NewPlanID = %q", id)
	}
	if again := domain.NewPlanID(fxNow, "alice|close #12"); again != id {
		t.Fatal("the id is a function of instant and seed")
	}
	if other := domain.NewPlanID(fxNow, "bob|close #12"); other == id {
		t.Fatal("a different seed must give a different id")
	}
	if later := domain.NewPlanID(fxNow.Add(time.Second), "alice|close #12"); later <= id {
		t.Fatal("ids must sort by creation time")
	}
}

func TestPlanCanonical(t *testing.T) {
	p := unsealedPlan()
	p.Hash = "stale"
	raw, err := p.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	cleared := p
	cleared.Hash = ""
	want, _ := json.Marshal(cleared)
	if string(raw) != string(want) {
		t.Fatalf("Canonical = %s", raw)
	}
	sum := sha256.Sum256(raw)
	hash, _ := p.ComputeHash()
	if hex.EncodeToString(sum[:]) != hash {
		t.Fatal("ComputeHash must hash the canonical form")
	}
}

func TestPlanSeal(t *testing.T) {
	sealed, err := unsealedPlan().Seal()
	if err != nil {
		t.Fatal(err)
	}
	if !sealed.ExpiresAt.Equal(fxNow.Add(domain.PlanExpiry)) {
		t.Fatalf("ExpiresAt = %s", sealed.ExpiresAt)
	}
	if sealed.Steps[0].Index != 0 || sealed.Steps[1].Index != 1 {
		t.Fatalf("steps must be reindexed: %+v", sealed.Steps)
	}
	if hash, _ := sealed.ComputeHash(); sealed.Hash != hash || len(hash) != 64 {
		t.Fatalf("Hash = %q, want %q", sealed.Hash, hash)
	}
	if err := sealed.Verify(); err != nil {
		t.Fatalf("a sealed plan verifies: %v", err)
	}
	explicit := unsealedPlan()
	explicit.ExpiresAt = fxNow.Add(time.Minute)
	sealed, _ = explicit.Seal()
	if !sealed.ExpiresAt.Equal(fxNow.Add(time.Minute)) {
		t.Fatal("an explicit expiry is kept")
	}
}

var sealErrorCases = []struct {
	name   string
	mutate func(*domain.Plan)
}{
	{"no creation time", func(p *domain.Plan) { p.CreatedAt = time.Time{} }},
	{"no steps", func(p *domain.Plan) { p.Steps = nil }},
}

func TestPlanSealErrors(t *testing.T) {
	for _, tc := range sealErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			p := unsealedPlan()
			tc.mutate(&p)
			if _, err := p.Seal(); !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("Seal error = %v, want ErrUsage", err)
			}
		})
	}
}

func TestPlanVerify(t *testing.T) {
	sealed, _ := unsealedPlan().Seal()
	tampered := sealed
	tampered.Steps = append([]domain.Step(nil), sealed.Steps...)
	tampered.Steps[1].After = "OPEN"
	err := tampered.Verify()
	if !errors.Is(err, domain.ErrApplyRefused) || domain.CodeOf(err) != domain.ExitApplyRefused {
		t.Fatalf("a modified plan is refused with exit code 6, got %v", err)
	}
	if !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("Verify error = %v", err)
	}
	unsigned := sealed
	unsigned.Hash = ""
	if err := unsigned.Verify(); !errors.Is(err, domain.ErrApplyRefused) {
		t.Fatalf("a plan without hash is refused, got %v", err)
	}
	upper := sealed
	upper.Hash = strings.ToUpper(sealed.Hash)
	if err := upper.Verify(); err != nil {
		t.Fatalf("hex case does not matter: %v", err)
	}
}

var applicableCases = []struct {
	name  string
	now   time.Time
	actor string
	want  string
}{
	{"fresh and same actor", fxNow.Add(time.Minute), fxViewer, ""},
	{"actor case does not matter", fxNow.Add(time.Minute), "Alice", ""},
	{"expired", fxNow.Add(domain.PlanExpiry), fxViewer, "expired at 2026-10-08T15:30:00Z"},
	{"other actor", fxNow.Add(time.Minute), "bob", "created by alice, current viewer is bob"},
}

func TestPlanCheckApplicable(t *testing.T) {
	sealed, _ := unsealedPlan().Seal()
	for _, tc := range applicableCases {
		t.Run(tc.name, func(t *testing.T) {
			err := sealed.CheckApplicable(tc.now, tc.actor)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("CheckApplicable = %v", err)
				}
				return
			}
			if !errors.Is(err, domain.ErrApplyRefused) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CheckApplicable = %v, want %q", err, tc.want)
			}
		})
	}
}
