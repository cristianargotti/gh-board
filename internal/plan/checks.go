package plan

import (
	"fmt"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// refuse wraps a reason in ErrApplyRefused (exit code 6).
func refuse(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, domain.ErrApplyRefused)...)
}

// checkPlan applies the refusals of section 6.4 in order: terminal, hash,
// expiry, actor, then the plan content itself. A dry run writes nothing,
// so it is the one mode allowed without a terminal.
func checkPlan(p domain.Plan, opts ApplyOptions, now time.Time) error {
	if !opts.DryRun && !opts.Interactive {
		return refuse("apply needs an interactive terminal, run it yourself")
	}
	if err := p.Verify(); err != nil {
		return err
	}
	if p.Actor == "" {
		return refuse("plan %s names no actor", p.ID)
	}
	if err := p.CheckApplicable(now, opts.Viewer.Login); err != nil {
		return err
	}
	return checkSteps(p)
}

// checkSteps refuses a plan with an unsafe id, no steps, out of order
// indexes or an operation this kit version cannot execute.
func checkSteps(p domain.Plan) error {
	if !validID(p.ID) {
		return refuse("plan id %q is not valid", p.ID)
	}
	if len(p.Steps) == 0 {
		return refuse("plan %s has no steps", p.ID)
	}
	for i, s := range p.Steps {
		if s.Index != i {
			return refuse("plan %s step %d carries index %d", p.ID, i, s.Index)
		}
		if !KnownOperation(s.Operation) {
			return refuse("plan %s step %d uses unknown operation %q", p.ID, i, s.Operation)
		}
	}
	return nil
}
