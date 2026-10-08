package domain

import "encoding/json"

// TokenChars is the approximate number of JSON characters per token, the
// rule of thumb that holds for English and PT-BR prose.
const TokenChars = 4

// EstimateTokens approximates the tokens a value costs in a prompt from
// the length of its JSON form.
func EstimateTokens(v any) int {
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return (len(raw) + TokenChars - 1) / TokenChars
}

// SectionShare is the share of one section in the budget of context: Cap
// is the most entries the first pass shows (zero for every entry) and
// Floor the fewest it keeps before the budget cuts entries below it in
// section order.
type SectionShare struct {
	Section string
	Cap     int
	Floor   int
}

// SnapshotShares are the shares of section 7, context: my items at most
// 10 at first, attention 8 to 12, the triage queue up to 5, epics all when
// they fit or at least 6, deliveries up to 6. The floors are kept under
// any budget, so a section with content never prints none; what remains
// of the budget goes to more of my items. The order is the order the
// budget lowers sections in, from the end.
var SnapshotShares = []SectionShare{
	{Section: SectionMine, Cap: 10, Floor: 10},
	{Section: SectionAttention, Cap: 12, Floor: 8},
	{Section: SectionTriage, Cap: 5, Floor: 5},
	{Section: SectionEpics, Cap: 0, Floor: 6},
	{Section: SectionDeliveries, Cap: 6, Floor: 6},
}

// FitBudget fits the snapshot to the budget measured on its JSON estimate.
func FitBudget(s *Snapshot, budget int) {
	FitBudgetWith(s, budget, nil)
}

// FitBudgetWith fits the snapshot to the budget measured by the caller:
// measure returns the tokens the snapshot costs in the output the caller
// emits, delimiters and layout included, so the budget bounds the bytes
// that leave the process. A nil measure estimates the JSON form. Every
// section gets its share (SnapshotShares): the budget lowers the epics
// and attention to their floors, from the end, and never below, so the
// shares are kept even when the budget is smaller than them; when the
// shares fit, the rest of the budget shows more of my items. Rules and
// sprint are never dropped and the omitted counts stay exact per section.
func FitBudgetWith(s *Snapshot, budget int, measure func(Snapshot) int) {
	if budget <= 0 {
		budget = DefaultContextBudget
	}
	if measure == nil {
		measure = func(snap Snapshot) int { return EstimateTokens(snap) }
	}
	full := *s
	keep := full.sizes().capped()
	fits := func() bool { return measure(full.take(keep)) <= budget }
	for !fits() {
		if !keep.lowerToFloor() {
			break
		}
	}
	if fits() {
		keep.growMine(len(full.Mine), fits)
	}
	*s = full.take(keep)
}

// growMine shows more of my items, one at a time, while they fit.
func (k shares) growMine(total int, fits func() bool) {
	for k[SectionMine] < total {
		k[SectionMine]++
		if !fits() {
			k[SectionMine]--
			return
		}
	}
}

// shares counts the entries kept per section.
type shares map[string]int

// sizes counts every entry of the droppable sections.
func (s Snapshot) sizes() shares {
	return shares{
		SectionMine: len(s.Mine), SectionAttention: len(s.Attention), SectionTriage: len(s.Triage),
		SectionEpics: len(s.Epics), SectionDeliveries: len(s.Deliveries),
	}
}

// capped applies the caps of the first pass.
func (k shares) capped() shares {
	for _, share := range SnapshotShares {
		if share.Cap > 0 && k[share.Section] > share.Cap {
			k[share.Section] = share.Cap
		}
	}
	return k
}

// lowerToFloor drops one entry of the last section still above its floor,
// epics before attention; false when every section is at its floor.
func (k shares) lowerToFloor() bool {
	for i := len(SnapshotShares) - 1; i >= 0; i-- {
		share := SnapshotShares[i]
		if k[share.Section] > share.Floor {
			k[share.Section]--
			return true
		}
	}
	return false
}

// take returns the snapshot cut to the shares, with the omitted counts
// and the completeness that follow from the cut.
func (s Snapshot) take(keep shares) Snapshot {
	out := s
	out.Mine = s.Mine[:keep[SectionMine]]
	out.Attention = s.Attention[:keep[SectionAttention]]
	out.Triage = s.Triage[:keep[SectionTriage]]
	out.Epics = s.Epics[:keep[SectionEpics]]
	out.Deliveries = s.Deliveries[:keep[SectionDeliveries]]
	out.Omitted, out.Completeness = map[string]int{}, Complete
	for section, total := range s.sizes() {
		out.MarkOmitted(section, total-keep[section])
	}
	return out
}
