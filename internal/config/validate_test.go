package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// expect is one mismatch the test looks for: path and value exact, reason
// as a substring, one command that must be listed.
type expect struct {
	path, value, reason, command string
}

type validateCase struct {
	name   string
	mutate func(cfg *domain.Config)
	schema func(s *config.Schema)
	want   []expect
}

var validateCases = []validateCase{
	{name: "reference file matches the board"},
	{
		name:   "field typo suggests the real name",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Lane.Field = "Frentes" },
		want:   []expect{{"capabilities.lane.field", "Frentes", `did you mean "Frente"`, "tidy"}},
	},
	{
		name:   "status option not on the field",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Status.Done = []string{"DONE!"} },
		want:   []expect{{"capabilities.status.done", "DONE!", `not an option of field "Status"; did you mean "DONE"`, "move"}},
	},
	{
		name:   "status field of the wrong kind",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Status.Field = "Sprint" },
		want:   []expect{{"capabilities.status.field", "Sprint", "field is ITERATION, expected SINGLE_SELECT", "status"}},
	},
	{
		name:   "sprint field of the wrong kind",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Sprint.Field = "Status" },
		want:   []expect{{"capabilities.sprint.field", "Status", "field is SINGLE_SELECT, expected ITERATION", "sprint set"}},
	},
	{
		name:   "empty field name",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Estimate.Field = "" },
		want:   []expect{{"capabilities.estimate.field", "", "field name is empty", "estimate"}},
	},
	{
		name: "epic field missing and issue type unknown",
		mutate: func(cfg *domain.Config) {
			cfg.Capabilities.Epic = &domain.EpicCapability{IssueType: "Story", Field: "Epic"}
		},
		want: []expect{
			{"capabilities.epic.field", "Epic", `did you mean "Épico"`, "epics"},
			{"capabilities.epic.issue_type", "Story", "issue type does not exist on the organization", "new epic"},
		},
	},
	{
		name:   "task issue type unknown",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Task.IssueType = "Tasks" },
		want:   []expect{{"capabilities.task.issue_type", "Tasks", `did you mean "Task"`, "new task"}},
	},
	{
		name: "blocked, triage and lab fields",
		mutate: func(cfg *domain.Config) {
			cfg.Capabilities.Blocked.Field = "Nope"
			cfg.Capabilities.Triage.DecisionField = "Title"
			cfg.Capabilities.Lab.GateField = "Gate"
		},
		want: []expect{
			{"capabilities.blocked.field", "Nope", "field does not exist on the board", "attention"},
			{"capabilities.triage.decision_field", "Title", "field is TITLE, expected SINGLE_SELECT", "new entry"},
			{"capabilities.lab.gate_field", "Gate", "field does not exist on the board", "item"},
		},
	},
	{
		name: "transition and wip options",
		mutate: func(cfg *domain.Config) {
			cfg.Policy.Transitions["QA"] = []string{"DONE", "Done"}
			cfg.Policy.WIP["Review"] = 1
		},
		want: []expect{
			{"policy.transitions", "QA", `not an option of field "Status"`, "move"},
			{"policy.transitions.QA", "Done", `did you mean "DONE"`, "move"},
			{"policy.wip", "Review", `not an option of field "Status"`, "attention"},
		},
	},
	{
		name:   "policy without status",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Status = nil },
		want: []expect{
			{"policy", "", "transitions and wip need capabilities.status", "move"},
			{"alerts.wip_exceeded", "status", "needs capabilities.status, which is not mapped", "attention"},
			{"alerts.stale_active", "status", "needs capabilities.status", "watch"},
			{"tidy.stamp_start_on_active", "status", "needs capabilities.status", "tidy"},
			{"tidy.epic_follows_tasks", "status", "needs capabilities.status", "tidy"},
		},
	},
	{
		name:   "alerts and tidy without the capability they need",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Sprint = nil; cfg.Capabilities.Dates = nil },
		want: []expect{
			{"alerts.overdue", "dates", "needs capabilities.dates", "watch"},
			{"alerts.sprint_ending", "sprint", "needs capabilities.sprint", "attention"},
			{"alerts.epic_without_dates", "dates", "needs capabilities.dates", "context"},
			{"tidy.sprint_from_target", "sprint", "needs capabilities.sprint", "tidy"},
			{"tidy.sprint_from_target", "dates", "needs capabilities.dates", "tidy"},
			{"tidy.stamp_start_on_active", "dates", "needs capabilities.dates", "tidy"},
		},
	},
	{
		name:   "time zone the runtime cannot load",
		mutate: func(cfg *domain.Config) { cfg.Timezone = "America/Sao_Paolo" },
		want:   []expect{{"timezone", "America/Sao_Paolo", "unknown IANA time zone", "digest"}},
	},
	{
		name:   "inherit from parent without lane",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Lane = nil },
		want:   []expect{{"tidy.inherit_from_parent", "lane", "needs capabilities.lane", "tidy"}},
	},
	{
		name:   "repository not linked",
		mutate: func(cfg *domain.Config) { cfg.Repository = "acme/ap" },
		want:   []expect{{"repository", "acme/ap", `not linked to the project; did you mean "acme/app"`, "new"}},
	},
	{
		name:   "another project than the discovered one",
		mutate: func(cfg *domain.Config) { cfg.Project = beta },
		want:   []expect{{"project", "beta/9", "the discovered project is acme/7", "all commands"}},
	},
	{
		name:   "labels missing on the repository",
		schema: func(s *config.Schema) { s.Labels = []domain.Label{{Name: "Entrada"}} },
		want: []expect{
			{"capabilities.triage.urgent_label", "urgente", "label does not exist on the repository", "new entry"},
			{"capabilities.lab.label", "lab", "label does not exist on the repository", "list --label"},
		},
	},
	{
		name:   "issue field missing on the organization",
		schema: func(s *config.Schema) { s.IssueFields = []string{"Start date"} },
		want:   []expect{{"capabilities.dates.target", "Target date", "issue field does not exist on the organization", "dates"}},
	},
	{
		name: "project dates need date fields",
		mutate: func(cfg *domain.Config) {
			cfg.Capabilities.Dates.Source = domain.DateSourceProject
			cfg.Capabilities.Dates.Target = "Sprint"
		},
		want: []expect{{"capabilities.dates.target", "Sprint", "field is ITERATION, expected DATE", "roadmap"}},
	},
	{
		name:   "unknown date source",
		mutate: func(cfg *domain.Config) { cfg.Capabilities.Dates.Source = "jira" },
		want:   []expect{{"capabilities.dates.source", "jira", "source must be issue_fields or project", "dates"}},
	},
	{
		name:   "member unknown",
		schema: func(s *config.Schema) { s.Members = []string{"Ana", "bruno"} },
		want:   []expect{{"digest.members", "carla", "login is not a member of the project", "digest"}},
	},
}

func TestValidateSchema(t *testing.T) {
	for _, tc := range validateCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, schema := validateInputs(t, tc)
			got, err := config.ValidateSchema(cfg, schema)
			if err != nil {
				t.Fatalf("ValidateSchema: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d mismatches %+v, want %d", len(got), got, len(tc.want))
			}
			for i, want := range tc.want {
				checkMismatch(t, got[i], want)
			}
		})
	}
}

// validateInputs applies the mutations of a case to the reference file and
// the full schema.
func validateInputs(t *testing.T, tc validateCase) (*domain.Config, config.Schema) {
	t.Helper()
	cfg := loadReference(t)
	if tc.mutate != nil {
		tc.mutate(cfg)
	}
	schema := fullSchema()
	if tc.schema != nil {
		tc.schema(&schema)
	}
	return cfg, schema
}

func checkMismatch(t *testing.T, got config.Mismatch, want expect) {
	t.Helper()
	if got.Path != want.path || got.Value != want.value {
		t.Fatalf("mismatch = %+v, want path %q value %q", got, want.path, want.value)
	}
	if !strings.Contains(got.Reason, want.reason) {
		t.Fatalf("reason %q does not contain %q", got.Reason, want.reason)
	}
	if !domain.Contains(got.Commands, want.command) {
		t.Fatalf("commands %v do not list %q", got.Commands, want.command)
	}
	assertNoDashes(t, got.Reason)
}

func TestValidateProjectOnly(t *testing.T) {
	cfg := loadReference(t)
	cfg.Capabilities.Epic.IssueType = "Story"
	cfg.Digest.Members = []string{"nobody"}
	got, err := config.Validate(cfg, boardProject())
	if err != nil || len(got) != 0 {
		t.Fatalf("labels, issue types and logins are unknown to a project: %+v, %v", got, err)
	}
	if _, err := config.ValidateSchema(nil, config.Schema{}); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("nil configuration must be a usage error, got %v", err)
	}
	empty, err := config.Validate(&domain.Config{Version: 1}, domain.Project{})
	if err != nil || len(empty) != 0 {
		t.Fatalf("an empty configuration has nothing to mismatch: %+v, %v", empty, err)
	}
}
