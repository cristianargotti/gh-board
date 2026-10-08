package commands

import (
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// readMatches applies the list filters on the client, so the result is
// the same whether or not the adapter pushed them down.
func readMatches(cfg *domain.Config, it domain.Item, f domain.ItemFilter, now time.Time, loc *time.Location) bool {
	return readMatchesValues(cfg, it, f) && readMatchesRules(cfg, it, f, now, loc)
}

// readMatchesValues compares the named values ignoring case.
func readMatchesValues(cfg *domain.Config, it domain.Item, f domain.ItemFilter) bool {
	caps := domain.CapabilitiesOf(cfg)
	typeName := ""
	if it.Issue.Type != nil {
		typeName = it.Issue.Type.Name
	}
	checks := []struct{ want, got string }{
		{f.Status, it.Text(domain.StatusFieldName(cfg))},
		{f.Lane, readCapabilityText(it, caps.Lane)},
		{f.Epic, readEpicText(it, caps.Epic)},
		{f.Sprint, readCapabilityText(it, caps.Sprint)},
		{f.Type, typeName},
	}
	for _, c := range checks {
		if c.want != "" && !strings.EqualFold(c.want, c.got) {
			return false
		}
	}
	if f.Assignee != "" && !it.AssignedTo(f.Assignee) {
		return false
	}
	return f.Label == "" || it.HasLabel(f.Label)
}

// readMatchesRules applies the state and the rule-based filters.
func readMatchesRules(cfg *domain.Config, it domain.Item, f domain.ItemFilter, now time.Time, loc *time.Location) bool {
	if it.Archived != f.Archived {
		return false
	}
	if f.State != "" && it.Issue.State != f.State {
		return false
	}
	if f.Overdue && !readIsOverdue(cfg, it, now, loc) {
		return false
	}
	if f.Blocked && !readIsBlocked(cfg, it) {
		return false
	}
	return !f.Triage || readIsTriage(cfg, it)
}

func readCapabilityText(it domain.Item, capability *domain.FieldCapability) string {
	if capability == nil {
		return ""
	}
	return it.Text(capability.Field)
}

func readEpicText(it domain.Item, capability *domain.EpicCapability) string {
	if capability == nil {
		return ""
	}
	return it.Text(capability.Field)
}

// readIsOverdue reports an open item whose target date is in the past.
func readIsOverdue(cfg *domain.Config, it domain.Item, now time.Time, loc *time.Location) bool {
	if !it.IsOpen() {
		return false
	}
	target, ok := domain.ItemDate(it, domain.CapabilitiesOf(cfg).Dates, domain.DateTarget, loc)
	return ok && domain.DaysFromToday(target, now, loc) < 0
}

// readIsBlocked reports a blocked field value or open blocking issues.
func readIsBlocked(cfg *domain.Config, it domain.Item) bool {
	if it.Issue.BlockedByCount > 0 {
		return true
	}
	return readCapabilityText(it, domain.CapabilitiesOf(cfg).Blocked) != ""
}

// readIsTriage reports an entry still waiting for a decision.
func readIsTriage(cfg *domain.Config, it domain.Item) bool {
	tri := domain.CapabilitiesOf(cfg).Triage
	if tri == nil || tri.Label == "" || !it.HasLabel(tri.Label) {
		return false
	}
	return tri.DecisionField == "" || it.Text(tri.DecisionField) == ""
}
