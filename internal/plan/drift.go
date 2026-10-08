package plan

import "github.com/cristianargotti/gh-board/internal/domain"

// An After value alone cannot prove this plan wrote it. Only unchanged
// preconditions are skipped; every other pending Before must still match.
func detectDrift(pending []domain.Step, snap Snapshot) (map[int]bool, []domain.Drift) {
	satisfied := make(map[int]bool)
	checked := make([]domain.Step, 0, len(pending))
	for _, s := range pending {
		if !domain.HasPrecondition(s.Operation) || isRef(s.Target.NodeID) {
			continue
		}
		if it, ok := snap.Items[s.Target.NodeID]; ok {
			current, _ := domain.StepCurrentValue(s, it)
			if current == s.Before && current == s.After {
				satisfied[s.Index] = true
				continue
			}
		}
		checked = append(checked, s)
	}
	return satisfied, domain.DetectDrift(domain.Plan{Steps: checked}, snap.Items)
}
