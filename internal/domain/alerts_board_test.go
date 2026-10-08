package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func evaluateAt(cfg *domain.Config, now time.Time, items ...domain.Item) []domain.Alert {
	return domain.EvaluateAlerts(domain.AlertInput{Config: cfg, Project: fixtureProject(), Items: items, Now: now})
}

func TestEvaluateWIPExceeded(t *testing.T) {
	items := []domain.Item{
		newItem(1, "a", withStatus(stActive, fxNow)), newItem(2, "b", withStatus(stActive, fxNow)),
		newItem(3, "c", withStatus(stActive, fxNow)), newItem(4, "d", withStatus(stTest, fxNow)),
		newItem(5, "e", withStatus(stTest, fxNow)), newItem(6, "closed", withStatus(stActive, fxNow), withClosed(fxNow)),
	}
	alerts := evaluateAt(fixtureConfig(), fxNow, items...)
	if len(alerts) != 1 {
		t.Fatalf("alerts = %+v", alerts)
	}
	a := alerts[0]
	if a.RuleID != domain.AlertWIPExceeded || a.Severity != domain.SeverityCritical || a.Fingerprint() != "wip_exceeded|" {
		t.Fatalf("alert = %+v", a)
	}
	if a.Message != "WIP excedido: IN PROGRESS 3/2, TEST / VALIDATION 2/1" {
		t.Fatalf("Message = %q", a.Message)
	}
	if got := evaluateAt(fixtureConfig(), fxNow, items[:2]...); len(got) != 0 {
		t.Fatalf("within the limits there is no alert: %+v", got)
	}
	cfg := fixtureConfig()
	cfg.Policy.WIP = nil
	if got := evaluateAt(cfg, fxNow, items...); len(got) != 0 {
		t.Fatalf("without limits there is no rule: %+v", got)
	}
}

var sprintEndingCases = []struct {
	name string
	now  time.Time
	days int
	want string
}{
	{"three days left is quiet", fxNow, 0, ""},
	{"one day left", fxNow.AddDate(0, 0, 2), 0, "Sprint 5 termina em 1 dia com 2 itens abertos"},
	{"last day", fxNow.AddDate(0, 0, 3).Add(11 * time.Hour), 0, "Sprint 5 termina hoje com 2 itens abertos"},
	{"days option widens the window", fxNow, 3, "Sprint 5 termina em 3 dias com 2 itens abertos"},
	{"between sprints", fxNow.AddDate(0, 0, 60), 0, ""},
}

func TestEvaluateSprintEnding(t *testing.T) {
	items := []domain.Item{
		newItem(1, "a", withField(fxSprint, sprintTitle)), newItem(2, "b", withField(fxSprint, sprintTitle)),
		newItem(3, "done", withField(fxSprint, sprintTitle), withStatus(stDone, fxNow)),
		newItem(4, "next", withField(fxSprint, "Sprint 6")),
	}
	for _, tc := range sprintEndingCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fixtureConfig()
			cfg.Alerts[domain.AlertSprintEnding] = domain.AlertRule{Days: tc.days}
			assertBoardAlert(t, evaluateAt(cfg, tc.now, items...), domain.AlertSprintEnding, domain.SeverityInfo, tc.want)
		})
	}
	single := evaluateAt(fixtureConfig(), fxNow.AddDate(0, 0, 2), items[0])
	assertBoardAlert(t, single, domain.AlertSprintEnding, domain.SeverityInfo, "Sprint 5 termina em 1 dia com 1 item aberto")
}

// assertBoardAlert expects exactly one board-level alert with the message,
// or none when the message is empty.
func assertBoardAlert(t *testing.T, alerts []domain.Alert, rule string, severity domain.Severity, message string) {
	t.Helper()
	if message == "" {
		if len(alerts) != 0 {
			t.Fatalf("alerts = %+v, want none", alerts)
		}
		return
	}
	if len(alerts) != 1 || alerts[0].RuleID != rule || alerts[0].Severity != severity || alerts[0].Item.NodeID != "" {
		t.Fatalf("alerts = %+v, want one %s", alerts, rule)
	}
	if alerts[0].Message != message {
		t.Fatalf("Message = %q, want %q", alerts[0].Message, message)
	}
}

func TestCurrentSprint(t *testing.T) {
	field, sprint, ok := domain.CurrentSprint(fixtureConfig(), fixtureProject(), fxNow)
	if !ok || field.Name != fxSprint || sprint.Title != sprintTitle {
		t.Fatalf("CurrentSprint = %+v, %+v, %v", field, sprint, ok)
	}
	if _, _, ok := domain.CurrentSprint(nil, fixtureProject(), fxNow); ok {
		t.Fatal("generic mode has no sprint")
	}
	if _, _, ok := domain.CurrentSprint(fixtureConfig(), domain.Project{}, fxNow); ok {
		t.Fatal("a missing field has no sprint")
	}
	if _, _, ok := domain.CurrentSprint(fixtureConfig(), fixtureProject(), fxNow.AddDate(1, 0, 0)); ok {
		t.Fatal("no iteration contains next year")
	}
}

var summaryCases = []struct {
	name   string
	sprint *domain.SprintSummary
	alerts []domain.Alert
	want   string
}{
	{"nothing", nil, nil, "sem alertas"},
	{"sprint only", &domain.SprintSummary{Title: sprintTitle, DaysLeft: 1}, nil, "Sprint 5 · 1 dia"},
	{"status line", &domain.SprintSummary{Title: sprintTitle, DaysLeft: 3}, []domain.Alert{
		{RuleID: domain.AlertOverdue}, {RuleID: domain.AlertOverdue}, {RuleID: domain.AlertBlocked},
	}, "Sprint 5 · 3 dias · 2 atrasadas · 1 bloqueada"},
	{"every rule", nil, []domain.Alert{
		{RuleID: domain.AlertOverdue},
		{RuleID: domain.AlertBlocked},
		{RuleID: domain.AlertBlocked},
		{RuleID: domain.AlertTriageSLA},
		{RuleID: domain.AlertWIPExceeded},
		{RuleID: domain.AlertStaleActive},
		{RuleID: domain.AlertStaleActive},
		{RuleID: domain.AlertEpicWithoutDates},
		{RuleID: domain.AlertEpicWithoutDates},
	}, "1 atrasada · 2 bloqueadas · 1 entrada fora do SLA · 1 WIP excedido · 2 paradas · 2 épicos sem datas"},
	{"sprint ending is implied by the days", nil, []domain.Alert{{RuleID: domain.AlertSprintEnding}}, "sem alertas"},
}

func TestAlertSummary(t *testing.T) {
	for _, tc := range summaryCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.AlertSummary(tc.sprint, tc.alerts); got != tc.want {
				t.Fatalf("AlertSummary = %q, want %q", got, tc.want)
			}
		})
	}
	if !reflect.DeepEqual(domain.AlertRuleIDs[:2], []string{domain.AlertOverdue, domain.AlertBlocked}) {
		t.Fatal("rule order changed")
	}
}
