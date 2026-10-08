package config

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// weekdays is the vocabulary of digest.weekday, as the section 5.2 example
// writes it.
var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// usage builds the one-line usage error every decoding check returns.
func usage(format string, args ...any) error {
	return fmt.Errorf("board.yml: %s: %w", fmt.Sprintf(format, args...), domain.ErrUsage)
}

// checkShape rejects declarations that decode but cannot mean anything:
// a half-named project, a malformed repository, a capability without its
// names, an option in two status classes, an unknown date source or
// weekday, and negative counts. A half declared capability is neither
// declared nor unavailable (principle 6), so it is an error.
func checkShape(cfg domain.Config) error {
	checks := []func(domain.Config) error{
		checkProjectShape, checkRepositoryShape, checkCapabilityNames,
		checkDatesSource, checkStatusClasses, checkWeekday, checkCounts, checkTemplate,
	}
	for _, check := range checks {
		if err := check(cfg); err != nil {
			return err
		}
	}
	return nil
}

// checkProjectShape accepts an absent project (the flag names it) but not
// a partial one, following the rules of domain.ParseProjectRef.
func checkProjectShape(cfg domain.Config) error {
	p := cfg.Project
	if p.IsZero() {
		return nil
	}
	if p.Owner == "" || strings.ContainsAny(p.Owner, "/# ") {
		return usage("project.owner %q must be a login without separators", p.Owner)
	}
	if p.Number <= 0 {
		return usage("project.number must be a positive integer, got %d", p.Number)
	}
	return nil
}

func checkRepositoryShape(cfg domain.Config) error {
	if cfg.Repository == "" {
		return nil
	}
	if _, _, err := domain.SplitRepository(cfg.Repository); err != nil {
		return fmt.Errorf("board.yml: %w", err)
	}
	return nil
}

// nameCheck is one key that must carry a name once its capability exists.
type nameCheck struct {
	path, value string
}

func checkCapabilityNames(cfg domain.Config) error {
	for _, n := range requiredNames(cfg.Capabilities) {
		if strings.TrimSpace(n.value) == "" {
			return usage("%s is required", n.path)
		}
	}
	return nil
}

// requiredNames lists the keys of every declared capability that name a
// field or a label. Issue types are optional: a user-owned board has none,
// and then epics are the items in the epic field and new creates plain
// issues.
func requiredNames(c domain.Capabilities) []nameCheck {
	var out []nameCheck
	add := func(path, value string) { out = append(out, nameCheck{path: path, value: value}) }
	if c.Status != nil {
		add("capabilities.status.field", c.Status.Field)
	}
	if c.Epic != nil {
		add("capabilities.epic.field", c.Epic.Field)
	}
	for _, f := range fieldCapabilities(c) {
		add("capabilities."+f.key+".field", f.cap.Field)
	}
	if c.Dates != nil {
		add("capabilities.dates.start", c.Dates.Start)
		add("capabilities.dates.target", c.Dates.Target)
	}
	if c.Triage != nil {
		add("capabilities.triage.label", c.Triage.Label)
		add("capabilities.triage.decision_field", c.Triage.DecisionField)
	}
	if c.Lab != nil {
		add("capabilities.lab.label", c.Lab.Label)
		add("capabilities.lab.gate_field", c.Lab.GateField)
		add("capabilities.lab.result_field", c.Lab.ResultField)
	}
	return out
}

// fieldCapability pairs a single-field capability with its key.
type fieldCapability struct {
	key string
	cap *domain.FieldCapability
}

// fieldCapabilities returns the declared single-field capabilities in the
// order of section 5.2.
func fieldCapabilities(c domain.Capabilities) []fieldCapability {
	all := []fieldCapability{{capLane, c.Lane}, {capSprint, c.Sprint}, {capEstimate, c.Estimate}, {capBlocked, c.Blocked}}
	out := all[:0]
	for _, f := range all {
		if f.cap != nil {
			out = append(out, f)
		}
	}
	return out
}

func checkDatesSource(cfg domain.Config) error {
	d := cfg.Capabilities.Dates
	if d == nil || d.Source == domain.DateSourceIssueFields || d.Source == domain.DateSourceProject {
		return nil
	}
	return usage("capabilities.dates.source must be %s or %s, got %q", domain.DateSourceIssueFields, domain.DateSourceProject, d.Source)
}

// checkStatusClasses rejects an option listed in two classes, because the
// class of such an option would be undefined.
func checkStatusClasses(cfg domain.Config) error {
	s := cfg.Capabilities.Status
	if s == nil {
		return nil
	}
	seen := map[string]string{}
	for _, class := range statusClasses(s) {
		for _, option := range class.names {
			if prev, ok := seen[option]; ok {
				if prev == class.name {
					return usage("capabilities.status.%s lists %q twice", class.name, option)
				}
				return usage("capabilities.status: option %q is listed in %s and %s", option, prev, class.name)
			}
			seen[option] = class.name
		}
	}
	return nil
}

// statusClass is one class of the status capability with its options.
type statusClass struct {
	name  string
	names []string
}

func statusClasses(s *domain.StatusCapability) []statusClass {
	return []statusClass{{"backlog", s.Backlog}, {"ready", s.Ready}, {"active", s.Active}, {"done", s.Done}}
}

func checkWeekday(cfg domain.Config) error {
	day := cfg.Digest.Weekday
	if day == "" || domain.Contains(weekdays, strings.ToLower(day)) {
		return nil
	}
	return usage("unknown digest weekday %q, expected one of %s", day, strings.Join(weekdays, ", "))
}

// count is one integer key of the file.
type count struct {
	path  string
	value int
}

// checkCounts rejects negative days, limits and thresholds; zero keeps
// the default of each rule.
func checkCounts(cfg domain.Config) error {
	for _, c := range counts(cfg) {
		if c.value < 0 {
			return usage("%s must not be negative, got %d", c.path, c.value)
		}
	}
	return nil
}

func counts(cfg domain.Config) []count {
	out := []count{{"policy.bulk_threshold", cfg.Policy.BulkThreshold}}
	if cfg.Capabilities.Task != nil {
		out = append(out, count{"capabilities.task.max_estimate_days", cfg.Capabilities.Task.MaxEstimateDays})
	}
	if cfg.Capabilities.Triage != nil {
		out = append(out, count{"capabilities.triage.sla_business_days", cfg.Capabilities.Triage.SLABusinessDays})
	}
	for _, id := range domain.AlertRuleIDs {
		if rule, ok := cfg.Alerts[id]; ok {
			out = append(out, count{"alerts." + id + ".days", rule.Days})
		}
	}
	for _, status := range sortedWIP(cfg.Policy.WIP) {
		out = append(out, count{"policy.wip." + status, cfg.Policy.WIP[status]})
	}
	return out
}
