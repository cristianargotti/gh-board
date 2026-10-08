package plan_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

// refusal mutates a plan or the options so that apply must refuse it.
type refusal struct {
	name string
	plan func(p *domain.Plan)
	opts func(o *plan.ApplyOptions)
	at   time.Time
}

var refusals = []refusal{
	{name: "no terminal", opts: func(o *plan.ApplyOptions) { o.Interactive = false }},
	{name: "hash mismatch", plan: func(p *domain.Plan) { p.Hash = "0000" }},
	{name: "edited after sealing", plan: func(p *domain.Plan) { p.Description = "Outra coisa." }},
	{name: "expired", at: now.Add(domain.PlanExpiry)},
	{name: "actor mismatch", opts: func(o *plan.ApplyOptions) { o.Viewer.Login = "someone" }},
	{name: "no actor", plan: func(p *domain.Plan) { p.Actor = ""; *p, _ = p.Seal() }},
	{name: "no steps", plan: func(p *domain.Plan) { p.Steps = nil; p.Hash, _ = p.ComputeHash() }},
	{name: "bad index", plan: func(p *domain.Plan) { p.Steps[0].Index = 4; p.Hash, _ = p.ComputeHash() }},
	{name: "unknown operation", plan: func(p *domain.Plan) { p.Steps[0].Operation = "delete_everything"; p.Hash, _ = p.ComputeHash() }},
	{name: "unsafe id", plan: func(p *domain.Plan) { p.ID = "../x"; p.Hash, _ = p.ComputeHash() }},
}

// refusedApply applies the mutated plan and returns the error, the writer
// and the state directory.
func refusedApply(t *testing.T, tc refusal) (*fakeWriter, string, error) {
	t.Helper()
	p := newPlan(t, closeStep())
	if tc.plan != nil {
		tc.plan(&p)
	}
	opts, _ := applyOptions(t)
	if tc.opts != nil {
		tc.opts(&opts)
	}
	w := newWriter()
	ports := newPorts(newReader(), w)
	if !tc.at.IsZero() {
		ports.Clock = fakeClock{tc.at}
	}
	_, err := plan.Apply(context.Background(), ports, p, opts)
	return w, opts.StateDir, err
}

func TestApplyRefusals(t *testing.T) {
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			w, dir, err := refusedApply(t, tc)
			if !errors.Is(err, domain.ErrApplyRefused) || domain.CodeOf(err) != domain.ExitApplyRefused {
				t.Fatalf("expected apply refused, got %v", err)
			}
			if len(w.calls) != 0 {
				t.Fatalf("nothing may be written, got %q", w.calls)
			}
			entries := auditOf(t, dir)
			if len(entries) != 1 || entries[0].Result != "refused" || !entries[0].At.Equal(now) && tc.at.IsZero() {
				t.Fatalf("audit = %+v", entries)
			}
		})
	}
}

func TestApplyDryRunNeedsNoTerminal(t *testing.T) {
	opts, out := applyOptions(t)
	opts.Interactive, opts.DryRun = false, true
	w := newWriter()
	result, err := plan.Apply(context.Background(), newPorts(newReader(), w), newPlan(t, closeStep()), opts)
	if err != nil || !result.DryRun || len(w.calls) != 0 {
		t.Fatalf("dry run = %+v, %v, calls %q", result, err, w.calls)
	}
	if entries := journalOf(t, opts.StateDir, result.PlanID); len(entries) != 0 {
		t.Fatalf("a dry run journals nothing, got %+v", entries)
	}
	if entries := auditOf(t, opts.StateDir); len(entries) != 1 || entries[0].Result != "dry-run" {
		t.Fatalf("audit = %+v", entries)
	}
	for _, want := range []string{"Dry run", "would run", "close_issue acme/repo#12"} {
		if !contains(out.String(), want) {
			t.Fatalf("output %q lacks %q", out.String(), want)
		}
	}
}

var portFailures = []struct {
	name  string
	ports func(p *plan.Ports)
	opts  func(o *plan.ApplyOptions)
}{
	{name: "no state dir", opts: func(o *plan.ApplyOptions) { o.StateDir = "" }},
	{name: "no reader", ports: func(p *plan.Ports) { p.Reader = nil }},
	{name: "no writer", ports: func(p *plan.Ports) { p.Writer = nil }},
	{name: "no clock", ports: func(p *plan.Ports) { p.Clock = nil }},
}

func TestApplyPortFailures(t *testing.T) {
	for _, tc := range portFailures {
		t.Run(tc.name, func(t *testing.T) {
			ports := newPorts(newReader(), newWriter())
			opts, _ := applyOptions(t)
			if tc.ports != nil {
				tc.ports(&ports)
			}
			if tc.opts != nil {
				tc.opts(&opts)
			}
			if _, err := plan.Apply(context.Background(), ports, newPlan(t, closeStep()), opts); !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
		})
	}
}
