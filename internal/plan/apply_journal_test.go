package plan_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

type interceptWriter struct {
	*fakeWriter
	beforeClose func()
}

func (w interceptWriter) CloseIssue(ctx context.Context, id string) error {
	w.beforeClose()
	return w.fakeWriter.CloseIssue(ctx, id)
}

func TestApplyReadsAllTargetsThenJournalsInOrderAndAuditsLast(t *testing.T) {
	opts, _ := applyOptions(t)
	opts.Out = nil
	r, w := newReader(), newWriter()
	p := newPlan(t, closeStep(), withTarget(closeStep(), "I_epic"))
	wrapped := interceptWriter{fakeWriter: w, beforeClose: func() {
		equalCalls(t, r.calls, []string{"DiscoverProject acme/7", "GetItem I_1", "GetItem I_epic"})
		if got := len(journalOf(t, opts.StateDir, p.ID)); got != len(w.calls) {
			t.Fatalf("journal has %d entries before write %d", got, len(w.calls))
		}
		if len(auditOf(t, opts.StateDir)) != 0 {
			t.Fatal("audit was appended before the final result")
		}
	}}
	result, err := plan.Apply(context.Background(), newPorts(r, wrapped), p, opts)
	if err != nil || len(result.Done) != 2 || result.Done[0] != 0 || result.Done[1] != 1 {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	equalCalls(t, w.calls, []string{"CloseIssue I_1", "CloseIssue I_epic"})
	entries := journalOf(t, opts.StateDir, p.ID)
	if len(entries) != 2 || entries[0].Step != 0 || entries[1].Step != 1 || len(auditOf(t, opts.StateDir)) != 1 {
		t.Fatalf("journal = %+v", entries)
	}
	r.calls = nil
	if _, err := plan.Apply(context.Background(), newPorts(r, wrapped), p, opts); err != nil {
		t.Fatal(err)
	}
	equalCalls(t, r.calls, nil)
	if len(w.calls) != 2 || len(journalOf(t, opts.StateDir, p.ID)) != 2 || len(auditOf(t, opts.StateDir)) != 2 {
		t.Fatal("completed resume must only add an audit line")
	}
}

func TestApplyStopsWhenJournalAppendFails(t *testing.T) {
	opts, _ := applyOptions(t)
	w := newWriter()
	p := newPlan(t, closeStep(), step(domain.OpAddComment, "", "Feito."))
	wrapped := interceptWriter{fakeWriter: w, beforeClose: func() {
		if err := os.MkdirAll(filepath.Join(opts.StateDir, plan.JournalDir, p.ID+".jsonl"), 0o700); err != nil {
			t.Fatal(err)
		}
	}}
	_, err := plan.Apply(context.Background(), newPorts(newReader(), wrapped), p, opts)
	if err == nil {
		t.Fatal("lost journal must stop apply")
	}
	equalCalls(t, w.calls, []string{"CloseIssue I_1"})
	if entries := auditOf(t, opts.StateDir); len(entries) != 1 || entries[0].Result != "failed" || entries[0].Error == "" {
		t.Fatalf("audit = %+v", entries)
	}
}

func TestApplyUnreadableJournalPreventsWrites(t *testing.T) {
	opts, _ := applyOptions(t)
	r, w := newReader(), newWriter()
	p := newPlan(t, closeStep())
	if err := os.MkdirAll(filepath.Join(opts.StateDir, plan.JournalDir, p.ID+".jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(context.Background(), newPorts(r, w), p, opts); err == nil {
		t.Fatal("unreadable journal must stop apply")
	}
	equalCalls(t, r.calls, nil)
	equalCalls(t, w.calls, nil)
	if entries := auditOf(t, opts.StateDir); len(entries) != 1 || entries[0].Result != "failed" {
		t.Fatalf("audit = %+v", entries)
	}
}
