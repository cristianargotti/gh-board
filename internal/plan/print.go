package plan

import (
	"fmt"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// printHeader names the plan and how many steps are still pending.
func (r *run) printHeader(pending int) {
	_, _ = fmt.Fprintf(r.opts.Out, "Plan %s (%s): %s\n", r.plan.ID, r.plan.Command, clean(r.plan.Description, 200))
	if pending == 0 {
		_, _ = fmt.Fprintln(r.opts.Out, "Every step is already done; nothing to apply.")
		return
	}
	if pending < len(r.plan.Steps) {
		_, _ = fmt.Fprintf(r.opts.Out, "Resuming: %d of %d steps pending.\n", pending, len(r.plan.Steps))
	}
}

// printDrifts lists every precondition that changed since the plan.
func (r *run) printDrifts(drifts []domain.Drift) {
	_, _ = fmt.Fprintln(r.opts.Out, "Drift detected; nothing was written:")
	for _, d := range drifts {
		_, _ = fmt.Fprintf(r.opts.Out, "  %s\n", clean(d.String(), 300))
	}
}

// printDryRun lists what a real apply would do with each pending step.
func (r *run) printDryRun(pending []domain.Step, satisfied map[int]bool) {
	_, _ = fmt.Fprintln(r.opts.Out, "Dry run; nothing was written:")
	for _, s := range pending {
		verdict := "would run"
		if satisfied[s.Index] {
			verdict = "already in place"
		}
		_, _ = fmt.Fprintf(r.opts.Out, "  %s [%s]\n", describe(s), verdict)
	}
}

// printStep reports one executed step.
func (r *run) printStep(step domain.Step, out Outcome, err error) {
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(r.opts.Out, "  %s [failed: %s]\n", describe(step), clean(err.Error(), 300))
	case out.Note != "":
		_, _ = fmt.Fprintf(r.opts.Out, "  %s [%s: %s]\n", describe(step), out.Status, clean(out.Note, 120))
	default:
		_, _ = fmt.Fprintf(r.opts.Out, "  %s [%s]\n", describe(step), out.Status)
	}
}

// printSummary closes the output of a finished apply.
func (r *run) printSummary(result ApplyResult) {
	_, _ = fmt.Fprintln(r.opts.Out, result.String())
}

// describe renders one step on a line: index, operation, target, field
// and the before and after values, with external text cleaned.
func describe(s domain.Step) string {
	line := fmt.Sprintf("%d. %s %s", s.Index+1, s.Operation, Label(s.Target))
	if s.Field != "" {
		line += " " + clean(s.Field, 60)
	}
	if domain.HasPrecondition(s.Operation) {
		return fmt.Sprintf("%s: %q -> %q", line, clean(s.Before, 80), clean(s.After, 80))
	}
	if s.After != "" {
		return fmt.Sprintf("%s: %s", line, clean(s.After, 80))
	}
	return line
}
