package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var driftItem = newItem(12, "Task twelve",
	withStatus(stActive, fxNow), withIssueField(fxTarget, "2026-10-20"),
	withAssignee("bob", "alice"), withLabel("entrada", "bug"), withMilestone("M1"), withParent(3, "Epic three"),
)

var currentValueCases = []struct {
	name  string
	step  domain.Step
	value string
	ok    bool
}{
	{"project field", domain.Step{Operation: domain.OpSetFieldValue, Field: fxStatus}, stActive, true},
	{"project field unset", domain.Step{Operation: domain.OpSetFieldValue, Field: fxLane}, "", true},
	{"issue field", domain.Step{Operation: domain.OpSetIssueFieldValue, Field: fxTarget}, "2026-10-20", true},
	{"close reads the state", domain.Step{Operation: domain.OpCloseIssue}, "OPEN", true},
	{"reopen reads the state", domain.Step{Operation: domain.OpReopenIssue}, "OPEN", true},
	{"assignees sorted", domain.Step{Operation: domain.OpAddAssignees}, "alice,bob", true},
	{"remove assignees", domain.Step{Operation: domain.OpRemoveAssignees}, "alice,bob", true},
	{"labels sorted", domain.Step{Operation: domain.OpAddLabels}, "bug,entrada", true},
	{"remove labels", domain.Step{Operation: domain.OpRemoveLabels}, "bug,entrada", true},
	{"title", domain.Step{Operation: domain.OpUpdateIssue, Field: domain.UpdateFieldTitle}, "Task twelve", true},
	{"body", domain.Step{Operation: domain.OpUpdateIssue, Field: domain.UpdateFieldBody}, "", true},
	{"milestone", domain.Step{Operation: domain.OpUpdateIssue, Field: domain.UpdateFieldMilestone}, "M1", true},
	{"unknown update field", domain.Step{Operation: domain.OpUpdateIssue, Field: "labels"}, "", true},
	{"issue type", domain.Step{Operation: domain.OpSetIssueType}, fxTaskType, true},
	{"parent", domain.Step{Operation: domain.OpAddSubIssue}, "acme/team-docs#3", true},
	{"archive state", domain.Step{Operation: domain.OpUnarchiveItem}, domain.ActiveValue, true},
	{"comment has no precondition", domain.Step{Operation: domain.OpAddComment}, "", false},
	{"creation has no precondition", domain.Step{Operation: domain.OpCreateIssue}, "", false},
}

func TestStepCurrentValue(t *testing.T) {
	for _, tc := range currentValueCases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := domain.StepCurrentValue(tc.step, driftItem)
			if value != tc.value || ok != tc.ok {
				t.Fatalf("StepCurrentValue = %q, %v, want %q, %v", value, ok, tc.value, tc.ok)
			}
			if domain.HasPrecondition(tc.step.Operation) != tc.ok {
				t.Fatalf("HasPrecondition = %v, want %v", domain.HasPrecondition(tc.step.Operation), tc.ok)
			}
		})
	}
}

func TestStepCurrentValueEmptyIssue(t *testing.T) {
	bare := domain.Item{Archived: true}
	if v, _ := domain.StepCurrentValue(domain.Step{Operation: domain.OpUpdateIssue, Field: domain.UpdateFieldMilestone}, bare); v != "" {
		t.Fatalf("no milestone = %q", v)
	}
	if v, _ := domain.StepCurrentValue(domain.Step{Operation: domain.OpSetIssueType}, bare); v != "" {
		t.Fatalf("no type = %q", v)
	}
	if v, _ := domain.StepCurrentValue(domain.Step{Operation: domain.OpAddSubIssue}, bare); v != "" {
		t.Fatalf("no parent = %q", v)
	}
	if v, _ := domain.StepCurrentValue(domain.Step{Operation: domain.OpUnarchiveItem}, bare); v != domain.ArchivedValue {
		t.Fatalf("archived = %q", v)
	}
	if domain.EncodeAssignees(nil) != "" || domain.EncodeLabels(nil) != "" {
		t.Fatal("empty sets encode to the empty string")
	}
}

func driftPlan() domain.Plan {
	target := domain.Target{NodeID: "I_12", Repository: "acme/team-docs", Number: 12, Title: "Task twelve"}
	return domain.Plan{ID: "p1", Steps: []domain.Step{
		{Index: 0, Operation: domain.OpSetFieldValue, Target: target, Field: fxStatus, Before: stActive, After: stDone},
		{Index: 1, Operation: domain.OpCloseIssue, Target: target, Before: "OPEN", After: "CLOSED"},
		{Index: 2, Operation: domain.OpAddComment, Target: domain.Target{NodeID: "I_99"}, After: "Fechada pelo plano"},
		{Index: 3, Operation: domain.OpAddLabels, Target: domain.Target{NodeID: "I_7", Repository: "acme/team-docs", Number: 7}, Before: "", After: "bug"},
	}}
}

func TestDetectDriftNone(t *testing.T) {
	current := map[string]domain.Item{"I_12": driftItem, "I_7": newItem(7, "seven")}
	if drifts := domain.DetectDrift(driftPlan(), current); len(drifts) != 0 {
		t.Fatalf("unexpected drift: %+v", drifts)
	}
	if err := domain.DriftError(nil); err != nil {
		t.Fatalf("no drift is no error: %v", err)
	}
}

func TestDetectDriftReportsEverything(t *testing.T) {
	moved := driftItem
	moved.Values = map[string]domain.FieldValue{fxStatus: {Field: fxStatus, Value: stTest}}
	current := map[string]domain.Item{"I_12": moved}
	drifts := domain.DetectDrift(driftPlan(), current)
	if len(drifts) != 2 {
		t.Fatalf("drifts = %+v", drifts)
	}
	first := drifts[0]
	if first.Step != 0 || first.Field != fxStatus || first.Expected != stActive || first.Actual != stTest || first.Missing {
		t.Fatalf("first drift = %+v", first)
	}
	if first.String() != `step 0 acme/team-docs#12 Status: expected "IN PROGRESS", found "TEST / VALIDATION"` {
		t.Fatalf("String = %q", first.String())
	}
	missing := drifts[1]
	if missing.Step != 3 || !missing.Missing || missing.String() != "step 3 acme/team-docs#7: target could not be read" {
		t.Fatalf("missing drift = %+v (%s)", missing, missing.String())
	}
	err := domain.DriftError(drifts)
	if !errors.Is(err, domain.ErrDrift) || domain.CodeOf(err) != domain.ExitDrift {
		t.Fatalf("DriftError = %v, code %v", err, domain.CodeOf(err))
	}
	if !strings.HasPrefix(err.Error(), "2 precondition(s) changed since the plan was written: step 0 ") || !strings.Contains(err.Error(), "; step 3 ") {
		t.Fatalf("DriftError = %v", err)
	}
}
