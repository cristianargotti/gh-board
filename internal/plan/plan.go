// Package plan writes, reads, lists and applies plan files and keeps the
// per-plan journal (section 6.4, ADR-003). A plan is an immutable JSON
// file under plans/ in the state directory; apply refuses it without a
// terminal, on hash mismatch, expiry or actor mismatch, re-reads every
// target before the first write, journals every step under journal/ and
// resumes from the journal on retry.
//
// The contract between plan producers and apply is the Step encoding:
// Before and After carry the values domain.StepCurrentValue reads, a
// creation step carries the JSON of its domain input in After, and a step
// may name the node created by an earlier step as "step:<index>".
package plan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// Layout of the state directory.
const (
	// PlansDir holds <id>.json under the state directory.
	PlansDir = "plans"
	// JournalDir holds <plan id>.jsonl under the state directory.
	JournalDir = "journal"
)

// Ports are the adapters apply needs.
type Ports struct {
	Reader domain.ProjectReader
	Writer domain.ProjectWriter
	Clock  domain.Clock
}

// ApplyOptions control apply. Interactive must be true (a terminal) or
// apply refuses with ErrApplyRefused; a dry run only needs the plan to be
// applicable, because it writes nothing.
type ApplyOptions struct {
	DryRun      bool
	Interactive bool
	Viewer      domain.Viewer
	StateDir    string
	Out         io.Writer
}

// ApplyResult reports which steps are done or skipped once apply returns,
// including the steps an earlier attempt completed.
type ApplyResult struct {
	PlanID  string
	Done    []int
	Skipped []int
	DryRun  bool
}

// Apply executes a plan: refuses without a terminal, on hash mismatch,
// expiry or actor mismatch; re-reads every target and stops before any
// write on drift; executes the steps in order writing one journal line
// per step; resumes from the journal on retry.
func Apply(ctx context.Context, ports Ports, p domain.Plan, opts ApplyOptions) (ApplyResult, error) {
	if err := checkPorts(ports, opts); err != nil {
		return ApplyResult{}, err
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	now := ports.Clock.Now()
	if err := checkPlan(p, opts, now); err != nil {
		return ApplyResult{}, record(opts, p, now, "refused", err)
	}
	release, err := audit.Lock(lockPath(opts.StateDir, p.ID))
	if errors.Is(err, audit.ErrLocked) {
		err = fmt.Errorf("plan %s is being applied by another process; remove %s if none is running: %w",
			p.ID, lockPath(opts.StateDir, p.ID), domain.ErrApplyRefused)
	}
	if err != nil {
		return ApplyResult{}, record(opts, p, now, "refused", err)
	}
	defer func() { _ = release() }()
	r := &run{ports: ports, plan: p, opts: opts, now: now, journal: NewJournal(opts.StateDir), created: map[int]string{}}
	result, err := r.execute(ctx)
	return result, record(opts, p, now, resultOf(result, err), err)
}

// lockPath is the lock file of one plan under the journal directory.
func lockPath(dir, id string) string {
	return filepath.Join(dir, JournalDir, id+".lock")
}

// checkPorts validates what apply needs before touching the plan.
func checkPorts(ports Ports, opts ApplyOptions) error {
	switch {
	case opts.StateDir == "":
		return fmt.Errorf("apply needs a state directory: %w", domain.ErrUsage)
	case ports.Reader == nil || ports.Writer == nil || ports.Clock == nil:
		return fmt.Errorf("apply needs a reader, a writer and a clock: %w", domain.ErrUsage)
	}
	return nil
}

// resultOf names the outcome for the audit line.
func resultOf(result ApplyResult, err error) string {
	switch {
	case errors.Is(err, domain.ErrDrift):
		return "drift"
	case errors.Is(err, domain.ErrApplyRefused):
		return "refused"
	case err != nil:
		return "failed"
	case result.DryRun:
		return "dry-run"
	default:
		return "applied"
	}
}

// record appends the audit line for the apply and returns the error the
// caller must report: the apply error when there is one, else an audit
// failure, so a lost audit line is never silent.
func record(opts ApplyOptions, p domain.Plan, now time.Time, result string, err error) error {
	entry := audit.Entry{
		At: now, Actor: opts.Viewer.Login, Host: opts.Viewer.Host, Project: p.Project,
		Command: "apply " + p.ID, Targets: p.Targets(), Items: p.Items(), Changes: changesOf(p),
		Result: result, DryRun: opts.DryRun, PlanID: p.ID,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	if auditErr := audit.Append(opts.StateDir, entry); auditErr != nil && err == nil {
		return fmt.Errorf("plan applied but the audit line failed: %w", auditErr)
	}
	return err
}

// changesOf lists the before and after value of every step for the audit.
func changesOf(p domain.Plan) []audit.Change {
	changes := make([]audit.Change, 0, len(p.Steps))
	for _, s := range p.Steps {
		changes = append(changes, audit.Change{Target: Label(s.Target), Field: s.Field, Before: s.Before, After: s.After})
	}
	return changes
}
