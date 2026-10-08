package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// shapeCases are files that decode but declare something that cannot
// mean anything; each must be a one-line usage error naming the key.
var shapeCases = []struct {
	name string
	yaml string
	want string
}{
	{"project without number", "version: 1\nproject: { owner: beta }\n", "project.number must be a positive integer"},
	{"project without owner", "version: 1\nproject: { number: 7 }\n", "project.owner"},
	{"project owner with separator", "version: 1\nproject: { owner: beta/x, number: 7 }\n", "project.owner"},
	{"repository without owner", "version: 1\nrepository: docs\n", "expected owner/name"},
	{"repository with spaces", "version: 1\nrepository: \"beta/my docs\"\n", "expected owner/name"},
	{"status without field", "version: 1\ncapabilities: { status: { done: [DONE] } }\n", "capabilities.status.field is required"},
	{"epic without field", "version: 1\ncapabilities: { epic: { issue_type: Feature } }\n", "capabilities.epic.field is required"},
	{"lane without field", "version: 1\ncapabilities: { lane: {} }\n", "capabilities.lane.field is required"},
	{"sprint with blank field", "version: 1\ncapabilities: { sprint: { field: \"  \" } }\n", "capabilities.sprint.field is required"},
	{"estimate without field", "version: 1\ncapabilities: { estimate: {} }\n", "capabilities.estimate.field is required"},
	{"blocked without field", "version: 1\ncapabilities: { blocked: {} }\n", "capabilities.blocked.field is required"},
	{"dates without start", "version: 1\ncapabilities: { dates: { target: T, source: project } }\n", "capabilities.dates.start is required"},
	{"dates without target", "version: 1\ncapabilities: { dates: { start: S, source: project } }\n", "capabilities.dates.target is required"},
	{"dates without source", "version: 1\ncapabilities: { dates: { start: S, target: T } }\n", "capabilities.dates.source must be issue_fields or project"},
	{"dates with another source", "version: 1\ncapabilities: { dates: { start: S, target: T, source: jira } }\n", `got "jira"`},
	{"triage without label", "version: 1\ncapabilities: { triage: { decision_field: D } }\n", "capabilities.triage.label is required"},
	{"triage without decision field", "version: 1\ncapabilities: { triage: { label: entrada } }\n", "capabilities.triage.decision_field is required"},
	{"lab without label", "version: 1\ncapabilities: { lab: { gate_field: G, result_field: R } }\n", "capabilities.lab.label is required"},
	{"lab without gate field", "version: 1\ncapabilities: { lab: { label: lab, result_field: R } }\n", "capabilities.lab.gate_field is required"},
	{"lab without result field", "version: 1\ncapabilities: { lab: { label: lab, gate_field: G } }\n", "capabilities.lab.result_field is required"},
	{"option in two classes", "version: 1\ncapabilities: { status: { field: Status, active: [QA], done: [QA] } }\n", `option "QA" is listed in active and done`},
	{"option twice in one class", "version: 1\ncapabilities: { status: { field: Status, done: [DONE, DONE] } }\n", `capabilities.status.done lists "DONE" twice`},
	{"unknown weekday", "version: 1\ndigest: { weekday: firday }\n", `unknown digest weekday "firday"`},
	{"negative bulk threshold", "version: 1\npolicy: { bulk_threshold: -1 }\n", "policy.bulk_threshold must not be negative"},
	{"negative wip", "version: 1\npolicy: { wip: { QA: -2 } }\n", "policy.wip.QA must not be negative"},
	{"negative estimate ceiling", "version: 1\ncapabilities: { task: { issue_type: Task, max_estimate_days: -3 } }\n", "capabilities.task.max_estimate_days must not be negative"},
	{"negative sla", "version: 1\ncapabilities: { triage: { label: a, decision_field: b, sla_business_days: -1 } }\n", "capabilities.triage.sla_business_days must not be negative"},
	{"negative alert days", "version: 1\nalerts: { stale_active: { days: -5 } }\n", "alerts.stale_active.days must not be negative"},
}

func TestDecodeShape(t *testing.T) {
	for _, tc := range shapeCases {
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

// shapeAccepted are edge cases the checks must let through: absent keys,
// zero counts that keep defaults, and the weekday in another case.
var shapeAccepted = []struct {
	name string
	yaml string
}{
	{"project absent", "version: 1\nrepository: beta/docs\n"},
	{"zero counts keep defaults", "version: 1\npolicy: { bulk_threshold: 0, wip: { QA: 0 } }\nalerts: { stale_active: {} }\n"},
	{"weekday in another case", "version: 1\ndigest: { weekday: Friday }\n"},
	{"triage without the optional keys", "version: 1\ncapabilities: { triage: { label: entrada, decision_field: D } }\n"},
	{"task without a ceiling", "version: 1\ncapabilities: { task: { issue_type: Task } }\n"},
}

func TestDecodeShapeAccepted(t *testing.T) {
	for _, tc := range shapeAccepted {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.Decode([]byte(tc.yaml)); err != nil {
				t.Fatalf("Decode: %v", err)
			}
		})
	}
}
