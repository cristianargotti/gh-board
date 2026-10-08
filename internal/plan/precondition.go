package plan

import "github.com/cristianargotti/gh-board/internal/domain"

// Precondition returns the live value a step's Before is compared with
// and whether the operation has one. The encoding is the domain's
// (StepCurrentValue), shared by plan producers, direct writes and apply.
func Precondition(step domain.Step, item domain.Item) (string, bool) {
	return domain.StepCurrentValue(step, item)
}

// needsItem reports whether apply re-reads the target of the operation:
// comments also target existing issues even though they have no Before.
func needsItem(op domain.Operation) bool {
	return domain.HasPrecondition(op) || op == domain.OpAddProjectItem || op == domain.OpAddComment
}
