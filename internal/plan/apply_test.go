package plan_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

func contains(text, want string) bool { return strings.Contains(text, want) }

func TestApplyClosesIssue(t *testing.T) {
	opts, out := applyOptions(t)
	w := newWriter()
	p := newPlan(t, closeStep())
	result, err := plan.Apply(context.Background(), newPorts(newReader(), w), p, opts)
	if err != nil || len(result.Done) != 1 || result.Done[0] != 0 || len(result.Skipped) != 0 {
		t.Fatalf("Apply = %+v, %v", result, err)
	}
	equalCalls(t, w.calls, []string{"CloseIssue I_1"})
	entries := journalOf(t, opts.StateDir, p.ID)
	if len(entries) != 1 || entries[0].Status != plan.StatusDone || entries[0].Step != 0 || !entries[0].At.Equal(now) {
		t.Fatalf("journal = %+v", entries)
	}
	log := auditOf(t, opts.StateDir)
	if len(log) != 1 || log[0].Result != "applied" || log[0].Actor != "octocat" || log[0].Changes[0].After != "CLOSED" {
		t.Fatalf("audit = %+v", log)
	}
	if !contains(out.String(), "[done]") || !contains(out.String(), "1 done, 0 skipped") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestApplyStopsOnDrift(t *testing.T) {
	opts, out := applyOptions(t)
	r, w := newReader(), newWriter()
	it := issueItem()
	it.Values["Status"] = domain.FieldValue{Field: "Status", Value: "In progress"}
	r.items["I_1"] = it
	steps := []domain.Step{step(domain.OpSetFieldValue, "Status", "Done"), closeStep()}
	p := newPlan(t, steps...)
	_, err := plan.Apply(context.Background(), newPorts(r, w), p, opts)
	if !errors.Is(err, domain.ErrDrift) || domain.CodeOf(err) != domain.ExitDrift {
		t.Fatalf("expected drift, got %v", err)
	}
	if len(w.calls) != 0 || len(journalOf(t, opts.StateDir, p.ID)) != 0 {
		t.Fatalf("drift must stop before any write: %q", w.calls)
	}
	if !contains(out.String(), "Drift detected") || !contains(out.String(), `expected "Todo", found "In progress"`) {
		t.Fatalf("output = %q", out.String())
	}
	if log := auditOf(t, opts.StateDir); log[0].Result != "drift" {
		t.Fatalf("audit = %+v", log)
	}
}

func TestApplyMissingTargetIsDrift(t *testing.T) {
	opts, _ := applyOptions(t)
	r := newReader()
	delete(r.items, "I_1")
	_, err := plan.Apply(context.Background(), newPorts(r, newWriter()), newPlan(t, closeStep()), opts)
	if !errors.Is(err, domain.ErrDrift) || !contains(err.Error(), "could not be read") {
		t.Fatalf("expected drift, got %v", err)
	}
}

func TestApplySkipsSatisfiedStep(t *testing.T) {
	opts, out := applyOptions(t)
	r, w := newReader(), newWriter()
	it := issueItem()
	it.Issue.State = domain.IssueClosed
	r.items["I_1"] = it
	s := closeStep()
	s.Before = string(domain.IssueClosed)
	p := newPlan(t, s)
	result, err := plan.Apply(context.Background(), newPorts(r, w), p, opts)
	if err != nil || len(result.Skipped) != 1 || len(w.calls) != 0 {
		t.Fatalf("Apply = %+v, %v, calls %q", result, err, w.calls)
	}
	if entries := journalOf(t, opts.StateDir, p.ID); entries[0].Status != plan.StatusSkipped || entries[0].Note == "" {
		t.Fatalf("journal = %+v", entries)
	}
	if !contains(out.String(), "already in place") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestApplyResumesFromJournal(t *testing.T) {
	opts, out := applyOptions(t)
	w := newWriter()
	p := newPlan(t, closeStep(), step(domain.OpAddComment, "", "Feito."))
	seed := plan.JournalEntry{PlanID: p.ID, Step: 0, Operation: domain.OpCloseIssue, Status: plan.StatusDone, At: now}
	if err := plan.NewJournal(opts.StateDir).Append(seed); err != nil {
		t.Fatal(err)
	}
	result, err := plan.Apply(context.Background(), newPorts(newReader(), w), p, opts)
	if err != nil || len(result.Done) != 2 {
		t.Fatalf("Apply = %+v, %v", result, err)
	}
	equalCalls(t, w.calls, []string{"AddComment I_1 Feito.\n\n" + plan.Marker(p.ID, 1)})
	if entries := journalOf(t, opts.StateDir, p.ID); len(entries) != 2 || entries[1].Created != "IC_1" {
		t.Fatalf("journal = %+v", entries)
	}
	if !contains(out.String(), "Resuming: 1 of 2") {
		t.Fatalf("output = %q", out.String())
	}
	result, err = plan.Apply(context.Background(), newPorts(newReader(), w), p, opts)
	if err != nil || len(result.Done) != 2 || len(w.calls) != 1 || !contains(out.String(), "nothing to apply") {
		t.Fatalf("second apply = %+v, %v, calls %q", result, err, w.calls)
	}
}

func TestApplyJournalsFailure(t *testing.T) {
	opts, out := applyOptions(t)
	w := newWriter()
	w.fail["CloseIssue"] = domain.ErrAPI
	p := newPlan(t, closeStep(), step(domain.OpAddComment, "", "Feito."))
	result, err := plan.Apply(context.Background(), newPorts(newReader(), w), p, opts)
	if !errors.Is(err, domain.ErrAPI) || len(result.Done) != 0 {
		t.Fatalf("Apply = %+v, %v", result, err)
	}
	equalCalls(t, w.calls, []string{"CloseIssue I_1"})
	entries := journalOf(t, opts.StateDir, p.ID)
	if len(entries) != 1 || entries[0].Status != plan.StatusFailed || !contains(entries[0].Error, "step 0") {
		t.Fatalf("journal = %+v", entries)
	}
	if !contains(out.String(), "[failed:") || auditOf(t, opts.StateDir)[0].Result != "failed" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestApplyFollowsCreatedReferences(t *testing.T) {
	opts, _ := applyOptions(t)
	w := newWriter()
	issue := domain.CreateIssueInput{RepositoryID: "R_1", Title: "Nova tarefa", IssueTypeID: "IT_task"}
	steps := []domain.Step{
		{Operation: domain.OpCreateIssue, Target: domain.Target{Repository: "acme/repo", Title: "Nova tarefa"}, After: payload(t, issue)},
		{Operation: domain.OpAddProjectItem, Target: domain.Target{NodeID: plan.Ref(0), Repository: "acme/repo"}},
		{Operation: domain.OpSetFieldValue, Target: domain.Target{NodeID: plan.Ref(0), ProjectItemID: plan.Ref(1)}, Field: "Status", After: "Todo"},
		{Operation: domain.OpAddComment, Target: domain.Target{NodeID: plan.Ref(0)}, After: "Criada pelo kit."},
	}
	p := newPlan(t, steps...)
	result, err := plan.Apply(context.Background(), newPorts(newReader(), w), p, opts)
	if err != nil || len(result.Done) != 4 {
		t.Fatalf("Apply = %+v, %v", result, err)
	}
	equalCalls(t, w.calls, []string{
		"CreateIssue R_1 Nova tarefa IT_task", "AddProjectItem PVT_1 I_new",
		"UpdateItemFieldValue PVT_1 PVTI_new F_status option=O_todo", "AddComment I_new Criada pelo kit.\n\n" + plan.Marker(p.ID, 3),
	})
	entries := journalOf(t, opts.StateDir, p.ID)
	if entries[0].Created != "I_new" || entries[1].Created != "PVTI_new" || entries[3].Created != "IC_1" {
		t.Fatalf("journal = %+v", entries)
	}
}

func TestApplyRefusesWhenLocked(t *testing.T) {
	opts, _ := applyOptions(t)
	p := newPlan(t, closeStep())
	release, err := audit.Lock(filepath.Join(opts.StateDir, plan.JournalDir, p.ID+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = release() }()
	_, err = plan.Apply(context.Background(), newPorts(newReader(), newWriter()), p, opts)
	if !errors.Is(err, domain.ErrApplyRefused) || !contains(err.Error(), "another process") {
		t.Fatalf("expected a lock refusal, got %v", err)
	}
}

func TestApplyReportsLostAuditLine(t *testing.T) {
	opts, _ := applyOptions(t)
	writeFile(t, filepath.Join(opts.StateDir, "journal", ".keep"), []byte(""))
	if err := audit.WriteAtomic(filepath.Join(opts.StateDir, audit.FileName, "x"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	_, err := plan.Apply(context.Background(), newPorts(newReader(), newWriter()), newPlan(t, closeStep()), opts)
	if err == nil || !contains(err.Error(), "audit line failed") {
		t.Fatalf("expected the audit failure, got %v", err)
	}
}

func TestApplyRereadFailure(t *testing.T) {
	opts, _ := applyOptions(t)
	r := newReader()
	r.fail["DiscoverProject"] = domain.ErrAPI
	if _, err := plan.Apply(context.Background(), newPorts(r, newWriter()), newPlan(t, closeStep()), opts); !errors.Is(err, domain.ErrAPI) {
		t.Fatalf("expected ErrAPI, got %v", err)
	}
}
