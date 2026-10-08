package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var templateFixture = &domain.Template{
	Labels: []domain.TemplateLabel{
		{Name: "entrada", Color: "0e8a16", Description: "Pedido novo"},
		{Name: " lab ", Color: "1d76db"},
	},
	Milestones: []domain.TemplateMilestone{
		{Title: "Onda 1", DueOn: "2026-11-30", Description: "Primeira entrega"},
		{Title: "Onda 2"},
	},
}

func TestTemplateInputs(t *testing.T) {
	labels := templateFixture.LabelInputs("R_1")
	if len(labels) != 2 || labels[0].RepositoryID != "R_1" || labels[0].Name != "entrada" || labels[0].Color != "0e8a16" || labels[0].Description != "Pedido novo" {
		t.Fatalf("labels = %+v", labels)
	}
	loc := time.FixedZone("BRT", -3*3600)
	milestones, err := templateFixture.MilestoneInputs("acme", "team-docs", loc)
	if err != nil || len(milestones) != 2 {
		t.Fatalf("milestones: %v, %+v", err, milestones)
	}
	first := milestones[0]
	if first.Owner != "acme" || first.Repo != "team-docs" || first.Title != "Onda 1" || first.Description != "Primeira entrega" || first.DueOn == nil {
		t.Fatalf("first = %+v", first)
	}
	if !first.DueOn.Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)) || milestones[1].DueOn != nil {
		t.Fatalf("due dates = %v %v", first.DueOn, milestones[1].DueOn)
	}
	if names := templateFixture.LabelNames(); len(names) != 2 || names[1] != "lab" {
		t.Fatalf("names = %v", names)
	}
}

func TestTemplateNilAndErrors(t *testing.T) {
	var none *domain.Template
	if none.LabelInputs("R_1") != nil || none.LabelNames() != nil {
		t.Fatal("a nil template declares nothing")
	}
	if got, err := none.MilestoneInputs("acme", "x", time.UTC); err != nil || got != nil {
		t.Fatalf("nil milestones: %v, %v", got, err)
	}
	bad := &domain.Template{Milestones: []domain.TemplateMilestone{{Title: "Onda", DueOn: "soon"}}}
	if _, err := bad.MilestoneInputs("acme", "x", time.UTC); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a malformed due date is a usage error: %v", err)
	}
}
