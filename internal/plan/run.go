package plan

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// run is one apply attempt of a plan.
type run struct {
	ports   Ports
	plan    domain.Plan
	opts    ApplyOptions
	now     time.Time
	journal *Journal
	created map[int]string
}

// execute resumes from the journal, re-reads the pending targets, stops
// on drift before any write, then runs the pending steps in order.
func (r *run) execute(ctx context.Context) (ApplyResult, error) {
	result := ApplyResult{PlanID: r.plan.ID, DryRun: r.opts.DryRun}
	pending, err := r.pending(&result)
	if err != nil {
		return result, err
	}
	r.printHeader(len(pending))
	if len(pending) == 0 {
		return result, nil
	}
	snap, err := RereadCreated(ctx, r.ports.Reader, r.plan, pending, r.createdIDs())
	if err != nil {
		return result, err
	}
	satisfied, drifts := detectDrift(pending, snap)
	if len(drifts) > 0 {
		r.printDrifts(drifts)
		return result, domain.DriftError(drifts)
	}
	if r.opts.DryRun {
		r.printDryRun(pending, satisfied)
		return result, nil
	}
	exec := NewExecutor(r.ports, r.plan, snap, r.opts.Viewer, r.now, r.created)
	return r.runSteps(ctx, exec, pending, satisfied, result)
}

// createdIDs lists the nodes earlier attempts created, by id, so the
// re-read can find an issue that is not on the board yet.
func (r *run) createdIDs() map[string]bool {
	ids := make(map[string]bool, len(r.created))
	for _, id := range r.created {
		ids[id] = true
	}
	return ids
}

// pending reads the journal and returns the steps still to run, filling
// the result and the created ids with what earlier attempts did.
func (r *run) pending(result *ApplyResult) ([]domain.Step, error) {
	entries, err := r.journal.Read(r.plan.ID)
	if err != nil {
		return nil, err
	}
	done := DoneSteps(entries)
	var pending []domain.Step
	for _, s := range r.plan.Steps {
		e, ok := done[s.Index]
		if !ok {
			s.Target, err = bindTarget(s, done)
			if err != nil {
				return nil, err
			}
			pending = append(pending, s)
			continue
		}
		if e.Created != "" {
			r.created[s.Index] = e.Created
		}
		result.add(e.Status, s.Index)
	}
	return pending, nil
}

// runSteps executes the pending steps in order, journaling each one, and
// stops at the first failure so a retry resumes from the journal.
func (r *run) runSteps(ctx context.Context, exec Executor, pending []domain.Step, satisfied map[int]bool, result ApplyResult) (ApplyResult, error) {
	for _, step := range pending {
		out, err := r.runStep(ctx, exec, step, satisfied[step.Index])
		if journalErr := r.journal.Append(r.entry(step, out, err)); journalErr != nil {
			err = errors.Join(err, journalErr)
		}
		if err != nil {
			r.printStep(step, out, err)
			return result, err
		}
		result.add(out.Status, step.Index)
		r.printStep(step, out, nil)
	}
	r.printSummary(result)
	return result, nil
}

// runStep skips unchanged preconditions without replaying a mutation.
func (r *run) runStep(ctx context.Context, exec Executor, step domain.Step, satisfied bool) (Outcome, error) {
	if satisfied {
		return Outcome{Status: StatusSkipped, Note: "already in place"}, nil
	}
	return exec.Execute(ctx, step)
}

// entry builds the journal line of a step.
func (r *run) entry(step domain.Step, out Outcome, err error) JournalEntry {
	e := JournalEntry{
		PlanID: r.plan.ID, Step: step.Index, Operation: step.Operation, Target: step.Target,
		Status: out.Status, Created: out.NodeID, Note: out.Note, At: r.ports.Clock.Now(),
	}
	if err != nil {
		e.Status = StatusFailed
		e.Error = err.Error()
	}
	return e
}

// add records a step outcome in the result, keeping the indexes sorted.
func (res *ApplyResult) add(status string, index int) {
	if status == StatusSkipped {
		res.Skipped = insertSorted(res.Skipped, index)
		return
	}
	res.Done = insertSorted(res.Done, index)
}

// insertSorted adds an index to a sorted list without duplicates.
func insertSorted(list []int, index int) []int {
	i := sort.SearchInts(list, index)
	if i < len(list) && list[i] == index {
		return list
	}
	list = append(list, 0)
	copy(list[i+1:], list[i:])
	list[i] = index
	return list
}

// String summarizes the result for a caller that prints it.
func (res ApplyResult) String() string {
	mode := "applied"
	if res.DryRun {
		mode = "dry run"
	}
	return fmt.Sprintf("plan %s %s: %d done, %d skipped", res.PlanID, mode, len(res.Done), len(res.Skipped))
}
