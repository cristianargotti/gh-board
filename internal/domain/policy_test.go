package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var transitionCases = []struct {
	name    string
	from    string
	to      string
	refused bool
	reason  string
}{
	{"allowed", stReady, stActive, false, ""},
	{"refused", stReady, stTest, true, "transição de READY TO DEV para TEST / VALIDATION não permitida; permitidas: IN PROGRESS"},
	{"backwards allowed by rule", stActive, stReady, false, ""},
	{"no rule for the source", stBacklog, stDone, false, ""},
	{"empty source", "", stActive, false, ""},
	{"same status", stReady, stReady, false, ""},
	{"rule with no destination", stDone, stActive, true, "transição de DONE para IN PROGRESS não permitida; permitidas: nenhuma"},
}

func policyWithClosedDone() domain.Policy {
	p := fixtureConfig().Policy
	p.Transitions[stDone] = []string{}
	return p
}

func TestCheckTransition(t *testing.T) {
	policy := policyWithClosedDone()
	for _, tc := range transitionCases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.CheckTransition(policy, tc.from, tc.to)
			if (err != nil) != tc.refused {
				t.Fatalf("CheckTransition error = %v, want refused %v", err, tc.refused)
			}
			if err == nil {
				return
			}
			var pe *domain.PolicyError
			if !errors.As(err, &pe) || pe.Kind != domain.PolicyTransition || pe.From != tc.from || pe.To != tc.to {
				t.Fatalf("PolicyError = %+v", pe)
			}
			if pe.Reason != tc.reason {
				t.Fatalf("Reason = %q, want %q", pe.Reason, tc.reason)
			}
		})
	}
}

var wipCases = []struct {
	name     string
	status   string
	occupied int
	refused  bool
}{
	{"below the limit", stActive, 1, false},
	{"at the limit", stActive, 2, true},
	{"above the limit", stActive, 3, true},
	{"no limit for the status", stReady, 50, false},
	{"zero limit disables the rule", "ZERO", 5, false},
}

func TestCheckWIP(t *testing.T) {
	policy := fixtureConfig().Policy
	policy.WIP["ZERO"] = 0
	for _, tc := range wipCases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.CheckWIP(policy, tc.status, tc.occupied)
			if (err != nil) != tc.refused {
				t.Fatalf("CheckWIP error = %v, want refused %v", err, tc.refused)
			}
			if err != nil && !strings.Contains(err.Error(), "limite de WIP em IN PROGRESS atingido") {
				t.Fatalf("unexpected reason: %v", err)
			}
		})
	}
	if _, ok := policy.WIPLimit("ZERO"); ok {
		t.Fatal("a zero limit is no limit")
	}
}

func TestCheckMoveAndExitCode(t *testing.T) {
	policy := fixtureConfig().Policy
	if err := domain.CheckMove(policy, stReady, stActive, 1); err != nil {
		t.Fatalf("allowed move refused: %v", err)
	}
	err := domain.CheckMove(policy, stReady, stTest, 0)
	var pe *domain.PolicyError
	if !errors.As(err, &pe) || pe.Kind != domain.PolicyTransition {
		t.Fatalf("transition must be checked first, got %v", err)
	}
	err = domain.CheckMove(policy, stReady, stActive, 2)
	if !errors.As(err, &pe) || pe.Kind != domain.PolicyWIP || pe.Status != stActive {
		t.Fatalf("WIP must be checked after the transition, got %v", err)
	}
	if !errors.Is(err, domain.ErrPolicy) || domain.CodeOf(err) != domain.ExitPolicy {
		t.Fatalf("every refusal maps to exit code 3, got %v", domain.CodeOf(err))
	}
	if !strings.HasPrefix(err.Error(), "refused by wip policy: ") {
		t.Fatalf("Error = %q", err.Error())
	}
}

func TestCountByStatusAndOccupancy(t *testing.T) {
	items := []domain.Item{
		newItem(1, "a", withStatus(stActive, fxNow)),
		newItem(2, "b", withStatus(stActive, fxNow)),
		newItem(3, "c", withStatus(stActive, fxNow), withClosed(fxNow)),
		newItem(4, "d", withStatus(stActive, fxNow), archived()),
		newItem(5, "e", withStatus(stTest, fxNow)),
		newItem(6, "f"),
	}
	counts := domain.CountByStatus(items, fxStatus)
	if counts[stActive] != 2 || counts[stTest] != 1 || len(counts) != 2 {
		t.Fatalf("CountByStatus = %v", counts)
	}
	if n := domain.WIPOccupied(items, fxStatus, stActive, "I_1"); n != 1 {
		t.Fatalf("WIPOccupied excluding the moved item = %d", n)
	}
	if n := domain.WIPOccupied(items, fxStatus, stActive, "I_5"); n != 2 {
		t.Fatalf("WIPOccupied = %d", n)
	}
	policy := domain.Policy{WIP: map[string]int{stTest: 1, stActive: 1, stReady: 0}}
	excess := policy.Exceeded(map[string]int{stActive: 3, stTest: 1, stReady: 9})
	if len(excess) != 1 || excess[0] != (domain.WIPExcess{Status: stActive, Count: 3, Limit: 1}) {
		t.Fatalf("Exceeded = %+v", excess)
	}
	policy.WIP[stTest] = 1
	excess = policy.Exceeded(map[string]int{stActive: 3, stTest: 2})
	if len(excess) != 2 || excess[0].Status != stActive || excess[1].Status != stTest {
		t.Fatalf("Exceeded must be sorted by status: %+v", excess)
	}
}

var bulkCases = []struct {
	threshold int
	n         int
	plan      bool
}{
	{0, 1000, false}, {10, 10, false}, {10, 11, true}, {1, 2, true},
}

func TestRequiresPlanAndPolicyOf(t *testing.T) {
	for _, tc := range bulkCases {
		p := domain.Policy{BulkThreshold: tc.threshold}
		if p.RequiresPlan(tc.n) != tc.plan {
			t.Errorf("threshold %d, n %d: RequiresPlan = %v", tc.threshold, tc.n, p.RequiresPlan(tc.n))
		}
	}
	if p := domain.PolicyOf(nil); p.BulkThreshold != 0 || p.Transitions != nil {
		t.Fatalf("PolicyOf(nil) = %+v", p)
	}
	if p := domain.PolicyOf(fixtureConfig()); p.BulkThreshold != 10 {
		t.Fatalf("PolicyOf = %+v", p)
	}
	if targets, ok := domain.PolicyOf(fixtureConfig()).AllowedTransitions(stReady); !ok || len(targets) != 1 {
		t.Fatalf("AllowedTransitions = %v, %v", targets, ok)
	}
}

var writePermissionCases = []struct {
	perm    domain.Permission
	refused bool
	mention string
}{
	{domain.PermissionAdmin, false, ""},
	{domain.PermissionWrite, false, ""},
	{domain.PermissionTriage, false, ""},
	{domain.PermissionRead, true, "permissão atual: READ"},
	{domain.Permission(""), true, "permissão atual: NONE"},
}

func TestCheckPermission(t *testing.T) {
	for _, tc := range writePermissionCases {
		err := domain.CheckPermission(tc.perm, "acme/team-docs")
		if (err != nil) != tc.refused {
			t.Fatalf("%s: error = %v", tc.perm, err)
		}
		if err == nil {
			continue
		}
		var pe *domain.PolicyError
		if !errors.As(err, &pe) || pe.Kind != domain.PolicyPermission || !strings.Contains(pe.Reason, "acme/team-docs") {
			t.Fatalf("%s: PolicyError = %+v", tc.perm, pe)
		}
		if !strings.Contains(pe.Reason, tc.mention) || domain.CodeOf(err) != domain.ExitPolicy {
			t.Fatalf("%s: reason %q, code %v", tc.perm, pe.Reason, domain.CodeOf(err))
		}
	}
}
