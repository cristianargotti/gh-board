package domain_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestEstimateTokens(t *testing.T) {
	if got := domain.EstimateTokens("abcd"); got != 2 {
		t.Fatalf("EstimateTokens = %d, want 2 for six JSON bytes", got)
	}
	if got := domain.EstimateTokens(map[string]int{"a": 1}); got != 2 {
		t.Fatalf("EstimateTokens = %d", got)
	}
	if got := domain.EstimateTokens(func() {}); got != 0 {
		t.Fatalf("an unmarshalable value costs nothing, got %d", got)
	}
}

func budgetSnapshot() domain.Snapshot {
	summary := func(n int) domain.ItemSummary {
		return domain.ItemSummary{Ref: "acme/team-docs#" + itoa(n), Title: "Item " + itoa(n)}
	}
	return domain.Snapshot{
		GeneratedAt: fxNow, Completeness: domain.Complete, Omitted: map[string]int{},
		Rules:      domain.TeamRules{Flow: []string{"READY TO DEV -> IN PROGRESS"}},
		Sprint:     &domain.SprintSummary{Title: sprintTitle, DaysLeft: 3},
		Mine:       []domain.ItemSummary{summary(1), summary(2)},
		Attention:  []domain.Alert{{RuleID: domain.AlertOverdue, Message: "Atrasada"}, {RuleID: domain.AlertBlocked, Message: "Bloqueada"}},
		Triage:     []domain.ItemSummary{summary(3), summary(4)},
		Epics:      []domain.EpicProgress{{Ref: "acme/team-docs#5", Title: "Epic"}, {Ref: "acme/team-docs#6", Title: "Epic"}},
		Deliveries: []domain.ItemSummary{summary(7), summary(8), summary(9)},
	}
}

// Every section of budgetSnapshot is under its floor, so no budget cuts
// it: the shares are kept and the snapshot stays complete.
func TestFitBudgetKeepsTheSharesUnderAnyBudget(t *testing.T) {
	for _, budget := range []int{1, 50, 100000} {
		s := budgetSnapshot()
		domain.FitBudget(&s, budget)
		if len(s.Deliveries) != 3 || len(s.Epics) != 2 || len(s.Triage) != 2 || len(s.Attention) != 2 || len(s.Mine) != 2 {
			t.Fatalf("budget %d cut a section under its floor: %+v", budget, s)
		}
		if s.Sprint == nil || len(s.Rules.Flow) != 1 {
			t.Fatal("rules and sprint are never dropped")
		}
		if s.Completeness != domain.Complete || len(s.Omitted) != 0 || s.OmittedTotal() != 0 {
			t.Fatalf("budget %d: Completeness = %s, Omitted = %v", budget, s.Completeness, s.Omitted)
		}
	}
	zero := budgetSnapshot()
	domain.FitBudget(&zero, 0)
	if zero.Completeness != domain.Complete {
		t.Fatal("a zero budget means the default budget")
	}
}

// sharesSnapshot holds more entries than every cap: 20 mine, 15 alerts,
// 8 triage entries, 10 epics and 9 deliveries.
func sharesSnapshot() domain.Snapshot {
	s := domain.Snapshot{Completeness: domain.Complete, Omitted: map[string]int{}}
	for i := 1; i <= 20; i++ {
		s.Mine = append(s.Mine, domain.ItemSummary{Ref: "mine#" + itoa(i)})
	}
	for i := 1; i <= 15; i++ {
		s.Attention = append(s.Attention, domain.Alert{RuleID: domain.AlertOverdue, Message: itoa(i)})
	}
	for i := 1; i <= 8; i++ {
		s.Triage = append(s.Triage, domain.ItemSummary{Ref: "triage#" + itoa(i)})
	}
	for i := 1; i <= 10; i++ {
		s.Epics = append(s.Epics, domain.EpicProgress{Ref: "epic#" + itoa(i)})
	}
	for i := 1; i <= 9; i++ {
		s.Deliveries = append(s.Deliveries, domain.ItemSummary{Ref: "done#" + itoa(i)})
	}
	return s
}

// entries measures a snapshot by its entry count, so a budget reads as
// a number of rows and the fitting is exact.
func entries(s domain.Snapshot) int {
	return len(s.Mine) + len(s.Attention) + len(s.Triage) + len(s.Epics) + len(s.Deliveries)
}

var shareCases = []struct {
	name    string
	budget  int
	mine    int
	alerts  int
	triage  int
	epics   int
	done    int
	omitted int
}{
	{"roomy: caps, all epics, then every mine item", 100, 20, 12, 5, 10, 6, 9},
	{"exact caps: no room to grow", 43, 10, 12, 5, 10, 6, 19},
	{"some room goes to mine", 50, 17, 12, 5, 10, 6, 12},
	{"epics lowered until they fit", 40, 10, 12, 5, 7, 6, 22},
	{"attention lowered to the floor", 35, 10, 8, 5, 6, 6, 27},
	{"the floors are kept under a smaller budget", 30, 10, 8, 5, 6, 6, 27},
	{"even under a budget of one", 1, 10, 8, 5, 6, 6, 27},
	{"zero means the default budget", 0, 20, 12, 5, 10, 6, 9},
}

func TestFitBudgetShares(t *testing.T) {
	for _, tc := range shareCases {
		t.Run(tc.name, func(t *testing.T) {
			s := sharesSnapshot()
			domain.FitBudgetWith(&s, tc.budget, entries)
			got := []int{len(s.Mine), len(s.Attention), len(s.Triage), len(s.Epics), len(s.Deliveries)}
			want := []int{tc.mine, tc.alerts, tc.triage, tc.epics, tc.done}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("sizes = %v, want %v", got, want)
			}
			assertSharesOmitted(t, s, tc.omitted, tc.mine, tc.done)
		})
	}
}

// assertSharesOmitted checks the omitted counts against the sizes of
// sharesSnapshot and that every section keeps its leading entries.
func assertSharesOmitted(t *testing.T, s domain.Snapshot, omitted, mine, done int) {
	t.Helper()
	if s.OmittedTotal() != omitted || (s.Completeness == domain.Complete) != (omitted == 0) {
		t.Fatalf("omitted = %v (%d), completeness %s", s.Omitted, s.OmittedTotal(), s.Completeness)
	}
	if s.Omitted[domain.SectionMine] != 20-mine || s.Omitted[domain.SectionDeliveries] != 9-done {
		t.Fatalf("per section omitted = %v", s.Omitted)
	}
	if s.Mine[0].Ref != "mine#1" || (done > 0 && s.Deliveries[0].Ref != "done#1") {
		t.Fatal("entries are kept from the front of every section")
	}
}

func TestSnapshotSharesFollowTheSectionOrder(t *testing.T) {
	if len(domain.SnapshotShares) != len(domain.SnapshotSections)-2 {
		t.Fatalf("SnapshotShares = %+v", domain.SnapshotShares)
	}
	for i, share := range domain.SnapshotShares {
		if share.Section != domain.SnapshotSections[i+2] || share.Floor <= 0 || (share.Cap > 0 && share.Cap < share.Floor) {
			t.Fatalf("share %d = %+v", i, share)
		}
	}
}
