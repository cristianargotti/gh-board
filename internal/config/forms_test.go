package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

const referenceForm = `name: Tarefa
description: Sub-issue de um épico, até 3 dias. Tipo Task.
title: "[T?] "
labels: ["entrada", "regra"]
type: Task
body:
  - type: input
    id: frente
    attributes:
      label: Frente
      description: Use o mesmo nome da opção do campo Frente no board. Frentes livres.
      placeholder: "Nome da frente"
  - type: input
    id: estimativa
    attributes:
      label: Estimativa (dias)
  - type: input
    id: datas
    attributes:
      description: Preencha também Start date e Target date na issue.
`

const referenceConfigForm = `blank_issues_enabled: true
contact_links:
  - name: Board do time
    url: https://github.com/orgs/beta/projects/46
    about: Fonte de verdade do time.
`

func teamConfig() *domain.Config {
	return &domain.Config{
		Version: 1,
		Capabilities: domain.Capabilities{
			Lane:     &domain.FieldCapability{Field: "Area"},
			Estimate: &domain.FieldCapability{Field: "Esforço (dias)"},
			Epic:     &domain.EpicCapability{IssueType: "Epic", Field: "Iniciativa"},
			Task:     &domain.TaskCapability{IssueType: "Story"},
			Dates:    &domain.DatesCapability{Start: "Inicio", Target: "Fim", Source: domain.DateSourceProject},
			Triage:   &domain.TriageCapability{Label: "triagem", DecisionField: "Decisao", UrgentLabel: "p0"},
		},
	}
}

func TestFormSubstitutionsFor(t *testing.T) {
	reference := loadReference(t)
	project := domain.Project{URL: "https://github.com/orgs/team/projects/9"}
	s := config.FormSubstitutionsFor(reference, teamConfig(), project, []domain.IssueType{{Name: "Epic"}, {Name: "Story"}})
	want := map[string]string{"Frente": "Area", "Estimativa (dias)": "Esforço (dias)", "Épico": "Iniciativa", "Start date": "Inicio", "Target date": "Fim", "entrada": "triagem", "urgente": "p0", "Decisão": "Decisao"}
	for from, to := range want {
		if s.Values[from] != to {
			t.Errorf("Values[%q] = %q, want %q", from, s.Values[from], to)
		}
	}
	if len(s.Values) != len(want) {
		t.Fatalf("values = %v", s.Values)
	}
	if s.IssueTypes["Feature"] != "Epic" || s.IssueTypes["Task"] != "Story" || len(s.KnownTypes) != 2 || s.BoardURL != project.URL {
		t.Fatalf("substitutions = %+v", s)
	}
	generic := config.FormSubstitutionsFor(reference, &domain.Config{Version: 1}, domain.Project{}, nil)
	if len(generic.Values) != 0 || len(generic.IssueTypes) != 0 || len(generic.KnownTypes) != 0 || generic.BoardURL != "" {
		t.Fatalf("an undeclared capability keeps the reference text: %+v", generic)
	}
}

func TestRenderForm(t *testing.T) {
	s := config.FormSubstitutions{
		Values:     map[string]string{"Frente": "Area", "Estimativa (dias)": "Esforço (dias)", "Start date": "Inicio", "Target date": "Fim", "entrada": "triagem"},
		IssueTypes: map[string]string{"Task": "Story"},
		KnownTypes: []string{"Story"},
		BoardURL:   "https://github.com/orgs/team/projects/9",
	}
	out, err := config.RenderForm("tarefa", []byte(referenceForm), s)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"type: Story", `labels: ["triagem", "regra"]`, "label: Area", "campo Area no board. Frentes livres.", "label: Esforço (dias)", "Preencha também Inicio e Fim na issue.", `placeholder: "Nome da frente"`, "Tipo Task."} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered form lacks %q:\n%s", want, text)
		}
	}
	cfg, err := config.RenderForm("config", []byte(referenceConfigForm), s)
	if err != nil || !strings.Contains(string(cfg), "url: https://github.com/orgs/team/projects/9") {
		t.Fatalf("config form: %v\n%s", err, cfg)
	}
	noTypes, err := config.RenderForm("tarefa", []byte(referenceForm), config.FormSubstitutions{})
	if err != nil || strings.Contains(string(noTypes), "type: Task") || !strings.Contains(string(noTypes), "  - type: input") {
		t.Fatalf("without issue types the top-level type line goes away: %v\n%s", err, noTypes)
	}
	kept, err := config.RenderForm("tarefa", []byte(referenceForm), config.FormSubstitutions{KnownTypes: []string{"Task"}})
	if err != nil || !strings.Contains(string(kept), "\ntype: Task\n") {
		t.Fatalf("a known type stays: %v\n%s", err, kept)
	}
}

func TestRenderFormRefusesBrokenYAML(t *testing.T) {
	s := config.FormSubstitutions{Values: map[string]string{"Frente": `Area "x": [`}}
	_, err := config.RenderForm("tarefa", []byte(referenceForm), s)
	if !errors.Is(err, domain.ErrUsage) || !strings.Contains(err.Error(), "form tarefa") {
		t.Fatalf("a name that breaks the YAML is refused: %v", err)
	}
	if _, err := config.RenderForm("empty", []byte("# nothing\n"), config.FormSubstitutions{}); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("an empty form is refused: %v", err)
	}
}

func TestRenderFormReplacesOnlyTheBoardLink(t *testing.T) {
	s := config.FormSubstitutions{BoardURL: "https://github.com/users/me/projects/3"}
	form := "body:\n  - type: markdown\n    attributes:\n      value: \"url: https://github.com/orgs/beta/projects/46\"\n    url: https://github.com/orgs/beta/projects/46\n    view: https://github.com/orgs/beta/projects/46/views/1\n"
	out, err := config.RenderForm("x", []byte(form), s)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Count(text, "https://github.com/users/me/projects/3") != 1 {
		t.Fatalf("only the url line is a board link:\n%s", text)
	}
	if !strings.Contains(text, "value: \"url: https://github.com/orgs/beta/projects/46\"") || !strings.Contains(text, "projects/46/views/1") {
		t.Fatalf("other lines keep their text:\n%s", text)
	}
}
