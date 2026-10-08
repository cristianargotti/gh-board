package commands

import (
	"context"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestPlanTidy(t *testing.T) {
	for _, mapped := range []bool{true, false} {
		t.Run(map[bool]string{true: "mapped", false: "skipped"}[mapped], func(t *testing.T) {
			planTestTidyCase(t, mapped)
		})
	}
}

func planTestTidyCase(t *testing.T, mapped bool) {
	t.Helper()
	deps, reader, writer, _ := planTestDeps(t)
	cfg := deps.Config.Config
	cfg.Tidy.StampStartOnActive = true
	cfg.Capabilities.Status = &domain.StatusCapability{Field: "Flow", Active: []string{"Doing"}}
	reader.items[0].Values["Flow"] = domain.FieldValue{Value: "Doing", UpdatedAt: planTestNow}
	if mapped {
		cfg.Capabilities.Dates = &domain.DatesCapability{Start: "Inicio", Source: domain.DateSourceIssueFields}
	}
	err := planTestExecute(context.Background(), []string{"tidy"}, deps)
	want := domain.ExitOK
	if mapped {
		want = domain.ExitPlanRequired
	}
	if domain.CodeOf(err) != want {
		t.Fatalf("result = %v", err)
	}
	if len(writer.calls) != 0 {
		t.Fatal("tidy wrote during planning")
	}
	if mapped && planTestSaved(t, deps).Steps[0].Operation != domain.OpSetIssueFieldValue {
		t.Fatal("wrong routine")
	}
}
