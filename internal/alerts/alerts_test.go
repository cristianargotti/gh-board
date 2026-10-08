package alerts_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

var (
	late    = alert(domain.AlertOverdue, domain.SeverityWarning, 1, "Atrasada há 3 dias")
	blocked = alert(domain.AlertBlocked, domain.SeverityWarning, 2, "Bloqueada por 1 issue aberta")
	wip     = boardAlert(domain.AlertWIPExceeded, domain.SeverityCritical, "WIP excedido: Em andamento 5/4")
	lateNow = alert(domain.AlertOverdue, domain.SeverityCritical, 1, "Atrasada há 7 dias")
)

type evalCase struct {
	name     string
	previous domain.AlertState
	current  domain.AlertState
	want     string
	err      string
}

var evalCases = []evalCase{
	{
		name: "everything is new without a previous state", current: stateOf(fxProject, late, blocked),
		want: "new=[overdue|I_1 blocked|I_2] escalated=[] resolved=[] unchanged=0",
	},
	{
		name: "a repeated state changes nothing", previous: stateOf(fxProject, late, blocked, wip), current: stateOf(fxProject, late, blocked, wip),
		want: "new=[] escalated=[] resolved=[] unchanged=3",
	},
	{
		name: "a severity that rose is escalated", previous: stateOf(fxProject, late), current: stateOf(fxProject, lateNow),
		want: "new=[] escalated=[overdue|I_1] resolved=[] unchanged=0",
	},
	{
		name: "a severity that fell is unchanged", previous: stateOf(fxProject, lateNow), current: stateOf(fxProject, late),
		want: "new=[] escalated=[] resolved=[] unchanged=1",
	},
	{
		name: "an alert that vanished is resolved", previous: stateOf(fxProject, late, blocked), current: stateOf(fxProject, blocked),
		want: "new=[] escalated=[] resolved=[overdue|I_1] unchanged=1",
	},
	{
		name: "a board-level alert diffs by rule", previous: stateOf(fxProject, wip), current: stateOf(fxProject, wip, late),
		want: "new=[overdue|I_1] escalated=[] resolved=[] unchanged=1",
	},
	{
		name: "another project knows nothing", previous: stateOf(fxOther, late, blocked), current: stateOf(fxProject, late),
		want: "new=[overdue|I_1] escalated=[] resolved=[] unchanged=0",
	},
	{
		name: "an unset project matches any", previous: stateOf(domain.ProjectRef{}, late), current: stateOf(fxProject, late),
		want: "new=[] escalated=[] resolved=[] unchanged=1",
	},
	{
		name: "owners compare without case", previous: stateOf(domain.ProjectRef{Owner: "ACME", Number: 7}, late), current: stateOf(fxProject, late),
		want: "new=[] escalated=[] resolved=[] unchanged=1",
	},
	{
		name: "a repeated fingerprint is an error", current: stateOf(fxProject, late, late),
		err: "current alert state: alert 1 repeats overdue|I_1",
	},
	{
		name: "an alert without a rule is an error", previous: stateOf(fxProject, domain.Alert{Item: domain.ItemRef{NodeID: "I_9"}}), current: stateOf(fxProject),
		err: "previous alert state: alert 0 has no rule id",
	},
}

// summarize renders a diff as fingerprints, for compact expectations.
func summarize(d alerts.Diff) string {
	prints := func(list []domain.Alert) string {
		out := make([]string, 0, len(list))
		for _, a := range list {
			out = append(out, a.Fingerprint())
		}
		return "[" + strings.Join(out, " ") + "]"
	}
	return fmt.Sprintf("new=%s escalated=%s resolved=%s unchanged=%d", prints(d.New), prints(d.Escalated), prints(d.Resolved), d.Unchanged)
}

func TestEvaluate(t *testing.T) {
	for _, tc := range evalCases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := alerts.Evaluate(tc.previous, tc.current)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got := summarize(diff); got != tc.want {
				t.Fatalf("diff = %s\nwant   %s", got, tc.want)
			}
		})
	}
}

var firstSeenCases = []struct {
	name string
	prev time.Time
	cur  time.Time
	want time.Time
}{
	{name: "the earlier instant is kept", prev: fxEarlier, cur: fxNow, want: fxEarlier},
	{name: "a zero current takes the previous", prev: fxEarlier, cur: time.Time{}, want: fxEarlier},
	{name: "a zero previous changes nothing", prev: time.Time{}, cur: fxNow, want: fxNow},
	{name: "a later previous is ignored", prev: fxNow.Add(time.Hour), cur: fxNow, want: fxNow},
}

func TestEvaluateCarriesFirstSeen(t *testing.T) {
	for _, tc := range firstSeenCases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := alerts.Evaluate(stateOf(fxProject, seen(late, tc.prev)), stateOf(fxProject, seen(lateNow, tc.cur)))
			if err != nil || len(diff.Escalated) != 1 {
				t.Fatalf("diff = %+v, %v", diff, err)
			}
			if got := diff.Escalated[0].FirstSeen; !got.Equal(tc.want) {
				t.Fatalf("FirstSeen = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDiffNotifiable(t *testing.T) {
	diff := alerts.Diff{New: []domain.Alert{blocked}, Escalated: []domain.Alert{lateNow}, Resolved: []domain.Alert{wip}}
	got := diff.Notifiable()
	if len(got) != 2 || got[0].RuleID != domain.AlertBlocked || got[1].RuleID != domain.AlertOverdue {
		t.Fatalf("Notifiable = %+v", got)
	}
	if alerts.MaxToastsPerHour != 5 || alerts.StateFileName != "alerts.json" || alerts.DefaultInterval != 10*time.Minute {
		t.Fatal("defaults changed")
	}
}
