package domain_test

import (
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Fixture names, sanitized: a board shaped like the reference board.yml of
// section 5.2 with generic logins.
const (
	fxStatus    = "Status"
	fxSprint    = "Sprint"
	fxLane      = "Frente"
	fxEpic      = "Épico"
	fxBlocked   = "Situação"
	fxDecision  = "Decisão"
	fxEstimate  = "Estimativa (dias)"
	fxStart     = "Start date"
	fxTarget    = "Target date"
	fxTriage    = "entrada"
	fxUrgent    = "urgente"
	fxEpicType  = "Feature"
	fxTaskType  = "Task"
	fxOwner     = "acme"
	fxRepo      = "team-docs"
	fxViewer    = "alice"
	fxZone      = "America/Sao_Paulo"
	stBacklog   = "BACKLOG"
	stReady     = "READY TO DEV"
	stActive    = "IN PROGRESS"
	stTest      = "TEST / VALIDATION"
	stDone      = "DONE"
	sprintTitle = "Sprint 5"
)

// fxNow is Thursday 2026-10-08 at noon in the board timezone.
var (
	fxLoc = mustLoc(fxZone)
	fxNow = time.Date(2026, 10, 8, 12, 0, 0, 0, fxLoc)
)

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func fixtureConfig() *domain.Config {
	return &domain.Config{
		Version: 1, Project: domain.ProjectRef{Owner: fxOwner, Number: 2},
		Repository: fxOwner + "/" + fxRepo, Language: "pt-BR", Timezone: fxZone,
		Capabilities: domain.Capabilities{
			Status: &domain.StatusCapability{
				Field: fxStatus, Backlog: []string{stBacklog}, Ready: []string{stReady},
				Active: []string{stActive, stTest}, Done: []string{stDone},
			},
			Epic:     &domain.EpicCapability{IssueType: fxEpicType, Field: fxEpic},
			Task:     &domain.TaskCapability{IssueType: fxTaskType, MaxEstimateDays: 3},
			Lane:     &domain.FieldCapability{Field: fxLane},
			Sprint:   &domain.FieldCapability{Field: fxSprint},
			Estimate: &domain.FieldCapability{Field: fxEstimate},
			Dates:    &domain.DatesCapability{Start: fxStart, Target: fxTarget, Source: domain.DateSourceIssueFields},
			Blocked:  &domain.FieldCapability{Field: fxBlocked},
			Triage:   &domain.TriageCapability{Label: fxTriage, DecisionField: fxDecision, SLABusinessDays: 2, UrgentLabel: fxUrgent},
		},
		Policy: domain.Policy{
			Transitions: map[string][]string{
				stReady: {stActive}, stActive: {stTest, stReady}, stTest: {stDone, stActive},
			},
			WIP: map[string]int{stActive: 2, stTest: 1}, BulkThreshold: 10,
		},
		Alerts: domain.AlertRules{
			domain.AlertOverdue: {}, domain.AlertBlocked: {}, domain.AlertTriageSLA: {},
			domain.AlertWIPExceeded: {}, domain.AlertSprintEnding: {Days: 2},
			domain.AlertStaleActive: {Days: 5}, domain.AlertEpicWithoutDates: {},
		},
		Rituals: []domain.Ritual{{Name: "Daily", When: "todo dia", Reads: []string{"active", "attention"}}},
	}
}

func fixtureProject() domain.Project {
	sprintStart := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	return domain.Project{
		Ref: domain.ProjectRef{Owner: fxOwner, Number: 2}, NodeID: "PVT_t9Unbd5vw8ZbxQsnTVHH", Title: "Team board",
		Fields: []domain.Field{
			{ID: "F_status", Name: fxStatus, DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{
				{ID: "o1", Name: stBacklog},
				{ID: "o2", Name: stReady},
				{ID: "o3", Name: stActive},
				{ID: "o4", Name: stTest},
				{ID: "o5", Name: stDone},
			}},
			{ID: "F_sprint", Name: fxSprint, DataType: domain.DataTypeIteration, Iterations: []domain.Iteration{
				{ID: "i4", Title: "Sprint 4", Start: sprintStart.AddDate(0, 0, -14), Duration: 14, Completed: true},
				{ID: "i5", Title: sprintTitle, Start: sprintStart, Duration: 14},
				{ID: "i6", Title: "Sprint 6", Start: sprintStart.AddDate(0, 0, 14), Duration: 14},
			}},
			{ID: "F_lane", Name: fxLane, DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{
				{ID: "l1", Name: "Plataforma"}, {ID: "l2", Name: "Produto"},
			}},
			{ID: "F_epic", Name: fxEpic, DataType: domain.DataTypeText},
			{ID: "F_blocked", Name: fxBlocked, DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{{ID: "b1", Name: "Bloqueada"}}},
			{ID: "F_decision", Name: fxDecision, DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{
				{ID: "d1", Name: "Ensinar"}, {ID: "d2", Name: "Encaminhar"},
			}},
			{ID: "F_estimate", Name: fxEstimate, DataType: domain.DataTypeNumber},
		},
		Repositories: []domain.Repository{{NodeID: "R_1", Owner: fxOwner, Name: fxRepo}},
		ViewerRole:   domain.RoleWriter,
	}
}

type itemOption func(*domain.Item)

// newItem builds an open task in the fixture repository.
func newItem(number int, title string, opts ...itemOption) domain.Item {
	it := domain.Item{
		ProjectItemID: "PVTI_" + itoa(number),
		Issue: domain.Issue{
			NodeID: "I_" + itoa(number), Owner: fxOwner, Repo: fxRepo, Number: number, Title: title,
			State: domain.IssueOpen, URL: "https://github.com/" + fxOwner + "/" + fxRepo + "/issues/" + itoa(number),
			Type: &domain.IssueType{ID: "IT_task", Name: fxTaskType}, CreatedAt: fxNow.AddDate(0, 0, -10),
		},
		Values: map[string]domain.FieldValue{},
	}
	for _, opt := range opts {
		opt(&it)
	}
	return it
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func withField(name, value string) itemOption {
	return func(it *domain.Item) { it.Values[name] = domain.FieldValue{Field: name, Value: value} }
}

func withStatus(value string, updatedAt time.Time) itemOption {
	return func(it *domain.Item) {
		it.Values[fxStatus] = domain.FieldValue{Field: fxStatus, Value: value, UpdatedAt: updatedAt}
	}
}

func withIssueField(name, value string) itemOption {
	return func(it *domain.Item) {
		it.IssueFields = append(it.IssueFields, domain.IssueFieldValue{FieldID: "IF_" + name, Name: name, Value: value})
	}
}

func withLabel(names ...string) itemOption {
	return func(it *domain.Item) {
		for _, n := range names {
			it.Issue.Labels = append(it.Issue.Labels, domain.Label{ID: "L_" + n, Name: n})
		}
	}
}

func withAssignee(logins ...string) itemOption {
	return func(it *domain.Item) {
		for _, l := range logins {
			it.Issue.Assignees = append(it.Issue.Assignees, domain.User{ID: "U_" + l, Login: l})
		}
	}
}

func withType(name string) itemOption {
	return func(it *domain.Item) { it.Issue.Type = &domain.IssueType{ID: "IT_" + name, Name: name} }
}

func withCreated(t time.Time) itemOption {
	return func(it *domain.Item) { it.Issue.CreatedAt = t }
}

func withClosed(t time.Time) itemOption {
	return func(it *domain.Item) {
		closed := t
		it.Issue.State = domain.IssueClosed
		it.Issue.ClosedAt = &closed
	}
}

func withBlockedBy(n int) itemOption {
	return func(it *domain.Item) { it.Issue.BlockedByCount = n }
}

func withSubIssues(total, completed int) itemOption {
	return func(it *domain.Item) {
		it.Issue.SubIssues = domain.SubIssueSummary{Total: total, Completed: completed, PercentCompleted: completed * 100 / max(total, 1)}
	}
}

func withParent(number int, title string) itemOption {
	return func(it *domain.Item) {
		it.Issue.Parent = &domain.ParentRef{NodeID: "I_" + itoa(number), Owner: fxOwner, Repo: fxRepo, Number: number, Title: title}
	}
}

func withMilestone(title string) itemOption {
	return func(it *domain.Item) {
		it.Issue.Milestone = &domain.Milestone{ID: "M_1", Number: 1, Title: title, State: "OPEN"}
	}
}

func archived() itemOption {
	return func(it *domain.Item) { it.Archived = true }
}

// day returns the YYYY-MM-DD form of fxNow shifted by days.
func day(offset int) string {
	return fxNow.AddDate(0, 0, offset).Format(domain.DateLayout)
}
