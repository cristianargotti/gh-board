package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestPaths(t *testing.T) {
	override := t.TempDir()
	dirs := config.Paths(override)
	if dirs.Config != filepath.Join(override, "config") || dirs.State != filepath.Join(override, "state") || dirs.Cache != filepath.Join(override, "cache") {
		t.Fatalf("override dirs = %+v", dirs)
	}
	native := config.Paths("")
	for _, d := range []string{native.Config, native.State, native.Cache} {
		if filepath.Base(d) != config.AppDir || !filepath.IsAbs(d) {
			t.Fatalf("native dir %q must be absolute and end with %s", d, config.AppDir)
		}
	}
	t.Setenv(config.EnvHome, override)
	if got := config.DefaultPaths(); got != dirs {
		t.Fatalf("DefaultPaths = %+v, want %+v", got, dirs)
	}
	t.Setenv(config.EnvHome, "")
	if got := config.DefaultPaths(); got != native {
		t.Fatalf("DefaultPaths without the override = %+v, want %+v", got, native)
	}
}

func TestHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.yml")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := config.Hash(path)
	if err != nil || got != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("Hash = %q, %v", got, err)
	}
	if _, err := config.Hash(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatal("missing file must fail")
	}
}

const validYAML = `version: 1
project: { owner: acme, number: 7 }
repository: acme/app
language: pt-BR
timezone: America/Sao_Paulo
capabilities:
  status: { field: Status, backlog: [BACKLOG], ready: [READY TO DEV], active: [IN PROGRESS, "TEST / VALIDATION"], done: [DONE] }
  epic: { issue_type: Feature, field: "Épico" }
  task: { issue_type: Task, max_estimate_days: 3 }
  lane: { field: Frente }
  sprint: { field: Sprint }
  estimate: { field: "Estimativa (dias)" }
  dates: { start: "Start date", target: "Target date", source: issue_fields }
  blocked: { field: "Situação" }
  triage: { label: entrada, decision_field: "Decisão", sla_business_days: 2, urgent_label: urgente }
  lab: { label: lab, gate_field: Porta, result_field: Resultado }
policy:
  transitions: { "READY TO DEV": [IN PROGRESS], "IN PROGRESS": [TEST / VALIDATION, READY TO DEV] }
  wip: { "IN PROGRESS": 6, "TEST / VALIDATION": 5 }
  bulk_threshold: 10
tidy: { inherit_from_parent: [lane, epic], sprint_from_target: true, stamp_start_on_active: true, epic_follows_tasks: true }
alerts: { overdue: {}, blocked: {}, triage_sla: {}, wip_exceeded: {}, sprint_ending: { days: 2 }, stale_active: { days: 5 }, epic_without_dates: {} }
digest: { weekday: friday, metrics: [first_response, autonomy, run_share], members: [ana, bruno] }
rituals:
  - { name: Daily, when: "todo dia, por chamada", reads: [active, attention, triage] }
`

func TestDecodeValid(t *testing.T) {
	cfg, err := config.Decode([]byte(validYAML))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Project.Number != 7 || cfg.Repository != "acme/app" || cfg.Language != "pt-BR" {
		t.Fatalf("header = %+v", cfg)
	}
	if cfg.Capabilities.Status == nil || !cfg.Capabilities.Status.IsDone("DONE") || cfg.Capabilities.Status.Classify("TEST / VALIDATION") != domain.StatusActive {
		t.Fatalf("status = %+v", cfg.Capabilities.Status)
	}
	if cfg.Capabilities.Dates.Source != domain.DateSourceIssueFields || cfg.Capabilities.Triage.SLABusinessDays != 2 || cfg.Capabilities.Task.MaxEstimateDays != 3 {
		t.Fatalf("capabilities = %+v", cfg.Capabilities)
	}
	if cfg.Policy.WIP["IN PROGRESS"] != 6 || cfg.Policy.BulkThreshold != 10 || len(cfg.Policy.Transitions["IN PROGRESS"]) != 2 {
		t.Fatalf("policy = %+v", cfg.Policy)
	}
	if cfg.Alerts["sprint_ending"].Days != 2 || len(cfg.Alerts) != 7 || !cfg.Tidy.EpicFollowsTasks {
		t.Fatalf("alerts/tidy = %+v %+v", cfg.Alerts, cfg.Tidy)
	}
	if cfg.Digest.Weekday != "friday" || len(cfg.Rituals) != 1 || cfg.Rituals[0].Reads[2] != "triage" {
		t.Fatalf("digest/rituals = %+v %+v", cfg.Digest, cfg.Rituals)
	}
}

var invalidYAML = map[string]string{
	"empty":           "",
	"unknown key":     "version: 1\npermissions: all\n",
	"unknown cap":     "version: 1\ncapabilities: { roadmap: { field: x } }\n",
	"unknown alert":   "version: 1\nalerts: { late: {} }\n",
	"unknown metric":  "version: 1\ndigest: { metrics: [velocity] }\n",
	"unknown inherit": "version: 1\ntidy: { inherit_from_parent: [sprint] }\n",
	"bad version":     "version: 2\n",
	"missing version": "repository: a/b\n",
	"malformed":       "version: [1\n",
	"wrong type":      "version: 1\npolicy: { bulk_threshold: many }\n",
}

func TestDecodeInvalid(t *testing.T) {
	for name, in := range invalidYAML {
		t.Run(name, func(t *testing.T) {
			_, err := config.Decode([]byte(in))
			if !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Fatalf("error must be one line: %q", err)
			}
		})
	}
}

func TestGeneric(t *testing.T) {
	if !(config.Loaded{Source: config.SourceGeneric}).Generic() || (config.Loaded{Source: config.SourceFlag}).Generic() {
		t.Fatal("Generic is wrong")
	}
}
