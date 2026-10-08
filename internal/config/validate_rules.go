package config

import (
	"sort"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// checkTimezone reports a time zone the runtime cannot load, because the
// kit would silently fall back to UTC and shift every date rule. The
// shipped binary needs the tzdata of the standard library on Windows for
// this check to pass there.
func (v *validator) checkTimezone() {
	tz := v.cfg.Timezone
	if tz == "" {
		return
	}
	if _, err := time.LoadLocation(tz); err != nil {
		v.add("timezone", tz, "unknown IANA time zone, dates would be read in UTC", cmdsTimezone)
	}
}

// checkPolicy validates transitions and WIP limits against the status
// options; without a status capability they cannot be checked.
func (v *validator) checkPolicy() {
	s := v.cfg.Capabilities.Status
	p := v.cfg.Policy
	if len(p.Transitions) == 0 && len(p.WIP) == 0 {
		return
	}
	if s == nil {
		v.add("policy", "", "transitions and wip need capabilities.status", cmdsPolicy)
		return
	}
	f, ok := v.schema.Project.FieldByName(s.Field)
	if !ok || f.DataType != domain.DataTypeSingleSelect {
		return
	}
	for _, from := range sortedKeys(p.Transitions) {
		v.option("policy.transitions", f, from, cmdsPolicy)
		for _, to := range p.Transitions[from] {
			v.option("policy.transitions."+from, f, to, cmdsPolicy)
		}
	}
	for _, name := range sortedWIP(p.WIP) {
		v.option("policy.wip", f, name, cmdsPolicy)
	}
}

// dependency is a key that only works when a capability is declared.
type dependency struct {
	path     string
	enabled  bool
	needs    []string
	commands []string
}

// checkDependencies reports alert rules and tidy steps that reference a
// capability the file does not declare, so that a rule never silently
// evaluates to nothing.
func (v *validator) checkDependencies() {
	for _, d := range v.dependencies() {
		if !d.enabled {
			continue
		}
		for _, need := range d.needs {
			if !v.has(need) {
				v.add(d.path, need, "needs capabilities."+need+", which is not mapped", d.commands)
			}
		}
	}
}

// dependencies lists what each alert rule and tidy step needs, in the
// evaluation order of the alert ids.
func (v *validator) dependencies() []dependency {
	needs := map[string][]string{
		domain.AlertOverdue:          {capDates},
		domain.AlertBlocked:          {capBlocked},
		domain.AlertTriageSLA:        {capTriage},
		domain.AlertWIPExceeded:      {capStatus},
		domain.AlertSprintEnding:     {capSprint},
		domain.AlertStaleActive:      {capStatus},
		domain.AlertEpicWithoutDates: {capEpic, capDates},
	}
	out := make([]dependency, 0, len(domain.AlertRuleIDs)+4)
	for _, id := range domain.AlertRuleIDs {
		_, enabled := v.cfg.Alerts[id]
		out = append(out, dependency{path: "alerts." + id, enabled: enabled, needs: needs[id], commands: cmdsAlerts})
	}
	t := v.cfg.Tidy
	return append(out,
		dependency{path: "tidy.inherit_from_parent", enabled: len(t.InheritFromParent) > 0, needs: t.InheritFromParent, commands: cmdsTidy},
		dependency{path: "tidy.sprint_from_target", enabled: t.SprintFromTarget, needs: []string{capSprint, capDates}, commands: cmdsTidy},
		dependency{path: "tidy.stamp_start_on_active", enabled: t.StampStartOnActive, needs: []string{capStatus, capDates}, commands: cmdsTidy},
		dependency{path: "tidy.epic_follows_tasks", enabled: t.EpicFollowsTasks, needs: []string{capEpic, capStatus}, commands: cmdsTidy},
	)
}

// has reports whether a capability is declared, by its key in board.yml.
func (v *validator) has(name string) bool {
	c := v.cfg.Capabilities
	switch name {
	case capStatus:
		return c.Status != nil
	case capEpic:
		return c.Epic != nil
	case capTask:
		return c.Task != nil
	case capLane:
		return c.Lane != nil
	case capSprint:
		return c.Sprint != nil
	case capEstimate:
		return c.Estimate != nil
	case capDates:
		return c.Dates != nil
	case capBlocked:
		return c.Blocked != nil
	case capTriage:
		return c.Triage != nil
	case capLab:
		return c.Lab != nil
	}
	return false
}

// checkMembers validates digest.members against the known logins when
// they were discovered.
func (v *validator) checkMembers() {
	if v.schema.Members == nil {
		return
	}
	for _, login := range v.cfg.Digest.Members {
		if containsFold(v.schema.Members, login) {
			continue
		}
		v.add("digest.members", login, withHint("login is not a member of the project", login, v.schema.Members), cmdsDigest)
	}
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedWIP(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func containsFold(list []string, value string) bool {
	for _, v := range list {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}
