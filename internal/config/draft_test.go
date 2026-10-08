package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

type draftCase struct {
	name     string
	in       config.DraftInput
	contains []string
	absent   []string
	check    func(t *testing.T, cfg *domain.Config)
}

func projectWith(fields ...domain.Field) domain.Project {
	return domain.Project{Ref: acme, Title: "Time Produto", Fields: fields}
}

var draftCases = []draftCase{
	{
		name: "full board maps status, sprint, dates and lists the issue types",
		in:   config.DraftInput{Project: boardProject(), Repository: "acme/app", IssueTypes: fullSchema().IssueTypes},
		contains: []string{
			"# board.yml draft written by gh board init from acme/7 (Time Produto).",
			"repository: acme/app # where new issues are created",
			"    field: Status\n    # Options on the board: BACKLOG, READY TO DEV, IN PROGRESS, TEST / VALIDATION, DONE\n",
			"    backlog: []\n    ready: []\n    active: []\n    done: []\n",
			"  sprint:\n    field: Sprint # the only iteration field on the board\n",
			"  dates:\n    start: Start date\n    target: Target date\n    source: project # detected from the field kind\n",
			"candidates for epic.issue_type and task.issue_type: Feature, Task\n",
			"# Unavailable until mapped by hand: epic, task, lane, estimate, blocked, triage, lab.\n",
		},
		check: func(t *testing.T, cfg *domain.Config) {
			t.Helper()
			if cfg.Capabilities.Status == nil || cfg.Capabilities.Status.Field != "Status" || len(cfg.Capabilities.Status.Done) != 0 {
				t.Fatalf("status = %+v", cfg.Capabilities.Status)
			}
			if cfg.Capabilities.Sprint == nil || cfg.Capabilities.Sprint.Field != "Sprint" || cfg.Capabilities.Dates.Source != domain.DateSourceProject {
				t.Fatalf("sprint/dates = %+v %+v", cfg.Capabilities.Sprint, cfg.Capabilities.Dates)
			}
			if cfg.Capabilities.Epic != nil || cfg.Capabilities.Task != nil || cfg.Capabilities.Lane != nil || cfg.Repository != "acme/app" {
				t.Fatalf("nothing else may be mapped: %+v", cfg.Capabilities)
			}
		},
	},
	{
		name: "empty board maps nothing",
		in:   config.DraftInput{Project: domain.Project{Ref: beta}},
		contains: []string{
			"from beta/9.\n",
			"# repository: owner/name # required for new and for the short form \"#n\"\n",
			"capabilities: {} # nothing could be mapped",
			"# No issue types were discovered on the organization",
			"# Unavailable until mapped by hand: status, epic, task, lane, sprint, estimate, dates, blocked, triage, lab.\n",
		},
		absent: []string{"status:", "sprint:", "dates:"},
		check: func(t *testing.T, cfg *domain.Config) {
			t.Helper()
			if cfg.Project != beta || cfg.Capabilities != (domain.Capabilities{}) || cfg.Repository != "" {
				t.Fatalf("decoded = %+v", cfg)
			}
		},
	},
	{
		name:   "two iteration fields are ambiguous",
		in:     config.DraftInput{Project: projectWith(field("Sprint", domain.DataTypeIteration), field("Ciclo", domain.DataTypeIteration))},
		absent: []string{"sprint:"},
	},
	{
		name:   "a Status field that is not a single select is not mapped",
		in:     config.DraftInput{Project: projectWith(field("Status", domain.DataTypeText))},
		absent: []string{"status:"},
	},
	{
		name:     "dates from organization issue fields",
		in:       config.DraftInput{Project: projectWith(selectField("Status", "Todo", "Done")), IssueFields: []string{"Start date", "Target date"}},
		contains: []string{"    source: issue_fields # detected from the field kind\n", "# Options on the board: Todo, Done\n"},
		check: func(t *testing.T, cfg *domain.Config) {
			t.Helper()
			if cfg.Capabilities.Dates == nil || cfg.Capabilities.Dates.Source != domain.DateSourceIssueFields {
				t.Fatalf("dates = %+v", cfg.Capabilities.Dates)
			}
		},
	},
	{
		name:   "only one date field present",
		in:     config.DraftInput{Project: projectWith(field("Start date", domain.DataTypeDate)), IssueFields: []string{"Target date"}},
		absent: []string{"dates:"},
	},
	{
		name:     "names that need quoting",
		in:       config.DraftInput{Project: projectWith(field("Sprint: atual", domain.DataTypeIteration), selectField("Status", "true", "@QA")), Repository: "beta/docs"},
		contains: []string{"field: 'Sprint: atual' #", "repository: beta/docs #", "# Options on the board: true, @QA\n"},
		check: func(t *testing.T, cfg *domain.Config) {
			t.Helper()
			if cfg.Capabilities.Sprint.Field != "Sprint: atual" || cfg.Repository != "beta/docs" {
				t.Fatalf("decoded = %+v %q", cfg.Capabilities.Sprint, cfg.Repository)
			}
		},
	},
	{
		name:     "title with line breaks stays on one line",
		in:       config.DraftInput{Project: domain.Project{Ref: beta, Title: "Board\nwith breaks"}},
		contains: []string{"from beta/9 (Board with breaks).\n"},
	},
}

func TestDraftFrom(t *testing.T) {
	for _, tc := range draftCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := config.DraftFrom(tc.in)
			if err != nil {
				t.Fatalf("DraftFrom: %v", err)
			}
			checkDraftText(t, string(out), tc)
			cfg, err := config.Decode(out)
			if err != nil {
				t.Fatalf("the draft must decode strictly: %v\n%s", err, out)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

// checkDraftText asserts the fragments a draft must and must not carry.
func checkDraftText(t *testing.T, text string, tc draftCase) {
	t.Helper()
	assertNoDashes(t, text)
	for _, want := range tc.contains {
		if !strings.Contains(text, want) {
			t.Fatalf("draft lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range tc.absent {
		if strings.Contains(text, unwanted) {
			t.Fatalf("draft must not contain %q:\n%s", unwanted, text)
		}
	}
}

func TestDraft(t *testing.T) {
	out, err := config.Draft(boardProject(), "")
	if err != nil || !strings.Contains(string(out), "# repository: owner/name") || !strings.Contains(string(out), "field: Status") {
		t.Fatalf("Draft = %v\n%s", err, out)
	}
	if _, err := config.Draft(domain.Project{}, "x/y"); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a project without a reference must be a usage error, got %v", err)
	}
	if _, err := config.Draft(boardProject(), "docs only"); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a repository that is not owner/name must be a usage error, got %v", err)
	}
}
