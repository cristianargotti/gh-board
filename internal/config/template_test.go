package config_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

const templateYAML = `version: 1
template:
  labels:
    - { name: entrada, color: "0e8a16", description: "Pedido novo para triagem" }
    - { name: urgente, color: "b60205" }
  milestones:
    - { title: "Onda 1", due_on: "2026-11-30", description: "Primeira entrega" }
    - { title: "Onda 2" }
`

func TestDecodeTemplate(t *testing.T) {
	cfg, err := config.Decode([]byte(templateYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Template == nil || len(cfg.Template.Labels) != 2 || len(cfg.Template.Milestones) != 2 {
		t.Fatalf("template = %+v", cfg.Template)
	}
	if l := cfg.Template.Labels[0]; l.Name != "entrada" || l.Color != "0e8a16" || l.Description != "Pedido novo para triagem" {
		t.Fatalf("label = %+v", l)
	}
	if m := cfg.Template.Milestones[0]; m.Title != "Onda 1" || m.DueOn != "2026-11-30" || m.Description != "Primeira entrega" {
		t.Fatalf("milestone = %+v", m)
	}
	if strings.Join(cfg.Template.LabelNames(), ",") != "entrada,urgente" {
		t.Fatalf("names = %v", cfg.Template.LabelNames())
	}
	plain, err := config.Decode([]byte("version: 1\n"))
	if err != nil || plain.Template != nil {
		t.Fatalf("an absent section stays nil: %v, %+v", err, plain.Template)
	}
}

var templateShapeCases = []struct {
	name string
	yaml string
	want string
}{
	{"unknown key", "version: 1\ntemplate: { colors: [] }\n", "field colors not found"},
	{"label without name", "version: 1\ntemplate: { labels: [{ color: \"0e8a16\" }] }\n", "template.labels[0].name is required"},
	{"label with blank name", "version: 1\ntemplate: { labels: [{ name: \"  \", color: \"0e8a16\" }] }\n", "template.labels[0].name is required"},
	{"label without color", "version: 1\ntemplate: { labels: [{ name: lab }] }\n", "template.labels[0].color must be six hex digits"},
	{"label with hash color", "version: 1\ntemplate: { labels: [{ name: lab, color: \"#0e8a16\" }] }\n", `got "#0e8a16"`},
	{"label twice", "version: 1\ntemplate: { labels: [{ name: lab, color: \"0e8a16\" }, { name: LAB, color: \"ffffff\" }] }\n", `template.labels lists "LAB" twice`},
	{"milestone without title", "version: 1\ntemplate: { milestones: [{ due_on: \"2026-11-30\" }] }\n", "template.milestones[0].title is required"},
	{"milestone bad date", "version: 1\ntemplate: { milestones: [{ title: Onda, due_on: \"30/11/2026\" }] }\n", "template.milestones[0].due_on must be YYYY-MM-DD"},
	{"milestone twice", "version: 1\ntemplate: { milestones: [{ title: Onda }, { title: onda }] }\n", `template.milestones lists "onda" twice`},
}

func TestDecodeTemplateShape(t *testing.T) {
	for _, tc := range templateShapeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Decode([]byte(tc.yaml))
			if !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "\n") {
				t.Fatalf("error %q must be one line and contain %q", err, tc.want)
			}
			assertNoDashes(t, err.Error())
		})
	}
}

// referenceTemplate is the embedded reference board.yml of the kit.
const referenceTemplate = "../../templates/board.yml"

func TestReferenceTemplateDecodes(t *testing.T) {
	data, err := os.ReadFile(referenceTemplate)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Template == nil || len(cfg.Template.Milestones) != 0 {
		t.Fatalf("template = %+v", cfg.Template)
	}
	names := cfg.Template.LabelNames()
	for _, want := range []string{"entrada", "urgente", "lab", "erro", "oportunidade", "regra", "sinal"} {
		if !domain.Contains(names, want) {
			t.Fatalf("template labels %v lack %q", names, want)
		}
	}
	if cfg.Capabilities.Triage == nil || !domain.Contains(names, cfg.Capabilities.Triage.Label) {
		t.Fatal("the triage label must be a template label")
	}
}
