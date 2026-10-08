package domain_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func snapshotItems() []domain.Item {
	lastWeek := fxNow.AddDate(0, 0, -7)
	return []domain.Item{
		newItem(1, "Mine active", withAssignee(fxViewer), withStatus(stActive, fxNow), withField(fxSprint, sprintTitle),
			withField(fxLane, "Plataforma"), withField(fxEpic, "Epic one"), withIssueField(fxTarget, day(-2))),
		newItem(2, "Bob ready", withAssignee("bob"), withStatus(stReady, fxNow), withField(fxSprint, sprintTitle)),
		newItem(3, "Mine but done", withAssignee(fxViewer), withStatus(stDone, fxNow), withField(fxSprint, sprintTitle)),
		newItem(4, "Delivered this week", withStatus(stDone, fxNow), withClosed(fxNow.AddDate(0, 0, -1))),
		newItem(5, "Delivered last week", withStatus(stDone, lastWeek), withClosed(lastWeek)),
		newItem(6, "Newer entry", withLabel(fxTriage), withCreated(fxNow.AddDate(0, 0, -1))),
		newItem(7, "Older entry", withLabel(fxTriage), withCreated(fxNow.AddDate(0, 0, -2))),
		newItem(8, "Decided entry", withLabel(fxTriage), withField(fxDecision, "Ensinar")),
		newItem(9, "Epic one", withType(fxEpicType), withAssignee("carol", "bob"), withStatus(stActive, fxNow),
			withSubIssues(4, 1), withIssueField(fxStart, day(-10)), withIssueField(fxTarget, day(20))),
	}
}

func buildFixtureSnapshot(forLogin string, alerts []domain.Alert) domain.Snapshot {
	return domain.BuildSnapshot(domain.SnapshotInput{
		Config: fixtureConfig(), Project: fixtureProject(), Items: snapshotItems(),
		Viewer: fxViewer, For: forLogin, Now: fxNow, Alerts: alerts, Budget: 100000,
	})
}

func TestBuildSnapshotHeader(t *testing.T) {
	s := buildFixtureSnapshot("", nil)
	if !s.GeneratedAt.Equal(fxNow) || s.Project != (domain.ProjectRef{Owner: fxOwner, Number: 2}) || s.Title != "Team board" || s.Viewer != fxViewer {
		t.Fatalf("header = %+v", s)
	}
	if s.Completeness != domain.Complete || len(s.Omitted) != 0 {
		t.Fatalf("a roomy budget is complete: %s %v", s.Completeness, s.Omitted)
	}
	if s.Sprint == nil || s.Sprint.Title != sprintTitle || s.Sprint.DaysLeft != 3 || s.Sprint.Open != 2 || s.Sprint.Done != 1 {
		t.Fatalf("Sprint = %+v", s.Sprint)
	}
	if s.Sprint.Start != "2026-09-28" || s.Sprint.End != "2026-10-11" {
		t.Fatalf("sprint bounds = %s, %s", s.Sprint.Start, s.Sprint.End)
	}
}

func TestBuildSnapshotSections(t *testing.T) {
	s := buildFixtureSnapshot("", nil)
	if len(s.Mine) != 1 || s.Mine[0].Ref != "acme/team-docs#1" {
		t.Fatalf("Mine = %+v", s.Mine)
	}
	mine := s.Mine[0]
	if mine.Status != stActive || mine.Lane != "Plataforma" || mine.Epic != "Epic one" || mine.Sprint != sprintTitle || mine.Target != day(-2) {
		t.Fatalf("Mine[0] = %+v", mine)
	}
	if !reflect.DeepEqual(mine.Assignees, []string{fxViewer}) || mine.NodeID != "I_1" || mine.URL == "" {
		t.Fatalf("Mine[0] identity = %+v", mine)
	}
	if len(s.Attention) != 1 || s.Attention[0].RuleID != domain.AlertOverdue || s.Attention[0].Item.NodeID != "I_1" {
		t.Fatalf("Attention is evaluated when nil: %+v", s.Attention)
	}
	if len(s.Triage) != 2 || s.Triage[0].Ref != "acme/team-docs#7" || s.Triage[1].Ref != "acme/team-docs#6" {
		t.Fatalf("Triage must be oldest first: %+v", s.Triage)
	}
	if len(s.Epics) != 1 {
		t.Fatalf("Epics = %+v", s.Epics)
	}
	want := domain.EpicProgress{Ref: "acme/team-docs#9", Title: "Epic one", Status: stActive, Owner: "carol", Total: 4, Completed: 1, Start: day(-10), Target: day(20)}
	if s.Epics[0] != want {
		t.Fatalf("Epics[0] = %+v, want %+v", s.Epics[0], want)
	}
	if len(s.Deliveries) != 1 || s.Deliveries[0].Ref != "acme/team-docs#4" {
		t.Fatalf("Deliveries = %+v", s.Deliveries)
	}
}

func TestBuildSnapshotForAndAlerts(t *testing.T) {
	given := []domain.Alert{{RuleID: domain.AlertBlocked, Message: "dada"}}
	s := buildFixtureSnapshot("bob", given)
	if len(s.Mine) != 2 || s.Mine[0].Ref != "acme/team-docs#9" || s.Mine[1].Ref != "acme/team-docs#2" || s.Viewer != fxViewer {
		t.Fatalf("--for selects another login, epics included, the dated item first: %+v", s.Mine)
	}
	if !reflect.DeepEqual(s.Attention, given) {
		t.Fatalf("given alerts are used as they are: %+v", s.Attention)
	}
	tight := domain.BuildSnapshot(domain.SnapshotInput{Config: fixtureConfig(), Project: fixtureProject(), Items: snapshotItems(), Viewer: fxViewer, Now: fxNow, Budget: 120})
	if tight.Completeness != domain.Complete || len(tight.Mine) != 1 || len(tight.Attention) != 1 || len(tight.Epics) != 1 {
		t.Fatalf("a tight budget keeps the shares: %s %v %+v", tight.Completeness, tight.Omitted, tight)
	}
}

func TestTeamRulesOf(t *testing.T) {
	if rules := domain.TeamRulesOf(nil); !reflect.DeepEqual(rules, domain.TeamRules{}) {
		t.Fatalf("generic mode has no rules: %+v", rules)
	}
	rules := domain.TeamRulesOf(fixtureConfig())
	want := domain.TeamRules{
		Flow:     []string{"IN PROGRESS -> TEST / VALIDATION | READY TO DEV", "READY TO DEV -> IN PROGRESS", "TEST / VALIDATION -> DONE | IN PROGRESS"},
		WIP:      []string{"IN PROGRESS: máximo 2", "TEST / VALIDATION: máximo 1"},
		SLA:      []string{"Triagem: decisão em 2 dias úteis; urgente: no mesmo dia", "Tarefa: estimativa máxima de 3 dias", "Escrita em lote acima de 10 itens exige plano"},
		Rituals:  []string{"Daily: todo dia (active, attention)"},
		Language: "pt-BR",
	}
	if !reflect.DeepEqual(rules, want) {
		t.Fatalf("TeamRulesOf = %+v, want %+v", rules, want)
	}
	bare := &domain.Config{Capabilities: domain.Capabilities{Triage: &domain.TriageCapability{Label: fxTriage, SLABusinessDays: 1}}}
	if sla := domain.TeamRulesOf(bare).SLA; !reflect.DeepEqual(sla, []string{"Triagem: decisão em 1 dias úteis"}) {
		t.Fatalf("SLA = %v", sla)
	}
}

func TestSnapshotHelpers(t *testing.T) {
	if domain.SprintSummaryOf(nil, fixtureProject(), nil, fxNow) != nil {
		t.Fatal("generic mode has no sprint summary")
	}
	generic := domain.SummarizeItem(nil, snapshotItems()[0])
	if generic.Status != stActive || generic.Lane != "" || generic.Epic != "" || generic.Sprint != "" || generic.Target != "" {
		t.Fatalf("generic summary = %+v", generic)
	}
	if got := domain.SummarizeItems(nil, nil); got == nil || len(got) != 0 {
		t.Fatal("SummarizeItems never returns nil")
	}
	if got := domain.TriageQueue(nil, snapshotItems()); len(got) != 0 {
		t.Fatalf("generic mode has no triage queue: %v", numbers(got))
	}
	if got := domain.EpicsProgress(nil, snapshotItems()); len(got) != 0 {
		t.Fatalf("generic mode has no epics: %+v", got)
	}
	delivered := domain.DeliveredSince(snapshotItems(), fxNow.AddDate(0, 0, -8))
	if !reflect.DeepEqual(numbers(delivered), []int{4, 5}) {
		t.Fatalf("DeliveredSince = %v", numbers(delivered))
	}
	noOwner := domain.EpicsProgress(fixtureConfig(), []domain.Item{newItem(1, "e", withType(fxEpicType))})
	if len(noOwner) != 1 || noOwner[0].Owner != "" || noOwner[0].Status != "" {
		t.Fatalf("an epic without assignees has no owner: %+v", noOwner)
	}
}

func TestSortByUrgency(t *testing.T) {
	items := []domain.Item{
		newItem(1, "Soon", withIssueField(fxTarget, day(5))),
		newItem(2, "Moved three days ago", withStatus(stActive, fxNow.AddDate(0, 0, -3))),
		newItem(3, "Overdue", withIssueField(fxTarget, day(-2))),
		newItem(4, "Moved yesterday", withStatus(stActive, fxNow.AddDate(0, 0, -1))),
		newItem(5, "Never moved"),
		newItem(6, "Overdue too", withIssueField(fxTarget, day(-2))),
	}
	got := numbers(domain.SortByUrgency(fixtureConfig(), items))
	if !reflect.DeepEqual(got, []int{3, 6, 1, 4, 2, 5}) {
		t.Fatalf("SortByUrgency = %v", got)
	}
	if !reflect.DeepEqual(numbers(items), []int{1, 2, 3, 4, 5, 6}) {
		t.Fatal("the input must not be reordered")
	}
	generic := numbers(domain.SortByUrgency(nil, items))
	if !reflect.DeepEqual(generic, []int{4, 2, 1, 3, 5, 6}) {
		t.Fatalf("without dates only the status activity orders: %v", generic)
	}
}

func TestSortBySeverity(t *testing.T) {
	if domain.SortBySeverity(nil) != nil {
		t.Fatal("nil stays nil")
	}
	alerts := []domain.Alert{
		{RuleID: domain.AlertOverdue, Severity: domain.SeverityWarning, Message: "w1"},
		{RuleID: domain.AlertBlocked, Severity: domain.SeverityInfo, Message: "i1"},
		{RuleID: domain.AlertTriageSLA, Severity: domain.SeverityCritical, Message: "c1"},
		{RuleID: domain.AlertWIPExceeded, Severity: domain.SeverityWarning, Message: "w2"},
		{RuleID: domain.AlertStaleActive, Severity: domain.SeverityCritical, Message: "c2"},
	}
	var got []string
	for _, a := range domain.SortBySeverity(alerts) {
		got = append(got, a.Message)
	}
	if !reflect.DeepEqual(got, []string{"c1", "c2", "w1", "w2", "i1"}) {
		t.Fatalf("SortBySeverity = %v", got)
	}
	if alerts[0].Message != "w1" {
		t.Fatal("the input must not be reordered")
	}
}
