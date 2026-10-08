package config

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Schema is everything doctor discovered that board.yml may name. The
// project is always known; the other lists are nil when they were not
// discovered, and a nil list skips its checks instead of reporting every
// name as missing.
type Schema struct {
	Project domain.Project
	// Labels of the configured repository.
	Labels []domain.Label
	// IssueTypes of the organization that owns the project.
	IssueTypes []domain.IssueType
	// IssueFields are the organization issue field names (dates.source
	// issue_fields).
	IssueFields []string
	// Members are the logins digest.members may name.
	Members []string
}

// Capability keys of section 5.2, as written in board.yml.
const (
	capStatus   = "status"
	capEpic     = "epic"
	capTask     = "task"
	capLane     = "lane"
	capSprint   = "sprint"
	capEstimate = "estimate"
	capDates    = "dates"
	capBlocked  = "blocked"
	capTriage   = "triage"
	capLab      = "lab"
)

// Verbs repeated across the dependency tables below.
const (
	verbAttention = "attention"
	verbContext   = "context"
	verbTidy      = "tidy"
	verbSet       = "set"
	verbDigest    = "digest"
)

// Commands that depend on each mapping, as doctor lists them next to a
// mismatch (section 5.2).
var (
	cmdsAll      = []string{"all commands"}
	cmdsRepo     = []string{"new", "the short form #n"}
	cmdsTimezone = []string{verbAttention, capSprint, verbDigest, "standup", verbTidy, "watch"}
	cmdsStatus   = []string{"move", verbSet, capStatus, "me", capSprint, verbAttention, verbContext, verbTidy, verbDigest}
	cmdsEpic     = []string{"epics", "roadmap", "new epic", "link", "list --epic", verbTidy}
	cmdsTask     = []string{"new task", capEstimate}
	cmdsLane     = []string{"new --lane", "list --lane", verbTidy}
	cmdsSprint   = []string{capSprint, "sprint set", "new --sprint", "list --sprint", verbAttention, verbTidy}
	cmdsEstimate = []string{capEstimate, "new --estimate"}
	cmdsDates    = []string{capDates, "new --start", "new --target", "roadmap", "epics", verbAttention, verbTidy}
	cmdsBlocked  = []string{verbAttention, "list --blocked", verbContext}
	cmdsTriage   = []string{"new entry", "list --triage", verbAttention, verbContext}
	cmdsLab      = []string{"item", verbSet, "list --label"}
	cmdsPolicy   = []string{"move", verbSet, verbAttention, verbContext}
	cmdsAlerts   = []string{verbAttention, "watch", verbContext}
	cmdsTidy     = []string{verbTidy}
	cmdsDigest   = []string{verbDigest, "standup"}
)

// Validate checks every field and option named in the file against the
// discovered project. Mismatches are returned, not errors; the error is
// for inputs that cannot be checked. Labels, issue types, issue fields and
// logins need ValidateSchema, because the project does not carry them.
func Validate(cfg *domain.Config, project domain.Project) ([]Mismatch, error) {
	return ValidateSchema(cfg, Schema{Project: project})
}

// ValidateSchema checks the file against everything doctor discovered.
// The order of the mismatches follows the order of the keys in board.yml.
func ValidateSchema(cfg *domain.Config, schema Schema) ([]Mismatch, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil, nothing to validate: %w", domain.ErrUsage)
	}
	v := &validator{cfg: cfg, schema: schema}
	v.checkProject()
	v.checkRepository()
	v.checkTimezone()
	v.checkStatus()
	v.checkRoles()
	v.checkDates()
	v.checkLabels()
	v.checkPolicy()
	v.checkDependencies()
	v.checkMembers()
	return v.out, nil
}

// validator accumulates mismatches while walking the configuration.
type validator struct {
	cfg    *domain.Config
	schema Schema
	out    []Mismatch
}

func (v *validator) add(path, value, reason string, commands []string) {
	v.out = append(v.out, Mismatch{Path: path, Value: value, Reason: reason, Commands: commands})
}

// checkProject reports a file that names another project than the one
// discovered, which happens after --project or a copied file.
func (v *validator) checkProject() {
	want, got := v.cfg.Project, v.schema.Project.Ref
	if want.IsZero() || got.IsZero() || want == got {
		return
	}
	v.add("project", want.String(), fmt.Sprintf("the discovered project is %s", got), cmdsAll)
}

// checkRepository requires the repository of new issues to be linked to
// the project, otherwise new items would never reach the board.
func (v *validator) checkRepository() {
	repo := v.cfg.Repository
	if repo == "" {
		return
	}
	names := make([]string, 0, len(v.schema.Project.Repositories))
	for _, r := range v.schema.Project.Repositories {
		if strings.EqualFold(r.FullName(), repo) {
			return
		}
		names = append(names, r.FullName())
	}
	v.add("repository", repo, withHint("repository is not linked to the project", repo, names), cmdsRepo)
}

// field checks that a field exists with the expected data type; an empty
// data type accepts any kind. It returns the field when it is usable.
func (v *validator) field(path, name string, want domain.DataType, commands []string) (domain.Field, bool) {
	if name == "" {
		v.add(path, name, "field name is empty", commands)
		return domain.Field{}, false
	}
	f, ok := v.schema.Project.FieldByName(name)
	if !ok {
		v.add(path, name, withHint("field does not exist on the board", name, v.schema.Project.FieldNames()), commands)
		return domain.Field{}, false
	}
	if want != "" && f.DataType != want {
		v.add(path, name, fmt.Sprintf("field is %s, expected %s", f.DataType, want), commands)
		return f, false
	}
	return f, true
}

// checkStatus validates the status field and every option of its classes.
func (v *validator) checkStatus() {
	s := v.cfg.Capabilities.Status
	if s == nil {
		return
	}
	f, ok := v.field("capabilities.status.field", s.Field, domain.DataTypeSingleSelect, cmdsStatus)
	if !ok {
		return
	}
	classes := []struct {
		name  string
		names []string
	}{{"backlog", s.Backlog}, {"ready", s.Ready}, {"active", s.Active}, {"done", s.Done}}
	for _, class := range classes {
		for _, n := range class.names {
			v.option("capabilities.status."+class.name, f, n, cmdsStatus)
		}
	}
}

// option reports a name that is not an option of a single select field.
func (v *validator) option(path string, f domain.Field, name string, commands []string) {
	if _, ok := f.OptionByName(name); ok {
		return
	}
	reason := fmt.Sprintf("not an option of field %q", f.Name)
	v.add(path, name, withHint(reason, name, f.OptionNames()), commands)
}

// checkRoles validates the single-field capabilities and their kinds.
func (v *validator) checkRoles() {
	c := v.cfg.Capabilities
	if c.Epic != nil {
		v.field("capabilities.epic.field", c.Epic.Field, "", cmdsEpic)
		if c.Epic.IssueType != "" {
			v.issueType("capabilities.epic.issue_type", c.Epic.IssueType, cmdsEpic)
		}
	}
	if c.Task != nil && c.Task.IssueType != "" {
		v.issueType("capabilities.task.issue_type", c.Task.IssueType, cmdsTask)
	}
	if c.Lane != nil {
		v.field("capabilities.lane.field", c.Lane.Field, domain.DataTypeSingleSelect, cmdsLane)
	}
	if c.Sprint != nil {
		v.field("capabilities.sprint.field", c.Sprint.Field, domain.DataTypeIteration, cmdsSprint)
	}
	if c.Estimate != nil {
		v.field("capabilities.estimate.field", c.Estimate.Field, domain.DataTypeNumber, cmdsEstimate)
	}
	if c.Blocked != nil {
		v.field("capabilities.blocked.field", c.Blocked.Field, domain.DataTypeSingleSelect, cmdsBlocked)
	}
	if c.Triage != nil {
		v.field("capabilities.triage.decision_field", c.Triage.DecisionField, domain.DataTypeSingleSelect, cmdsTriage)
	}
	if c.Lab != nil {
		v.field("capabilities.lab.gate_field", c.Lab.GateField, domain.DataTypeSingleSelect, cmdsLab)
		v.field("capabilities.lab.result_field", c.Lab.ResultField, domain.DataTypeSingleSelect, cmdsLab)
	}
}

// issueType checks a name against the organization issue types when they
// were discovered.
func (v *validator) issueType(path, name string, commands []string) {
	if v.schema.IssueTypes == nil {
		return
	}
	names := make([]string, 0, len(v.schema.IssueTypes))
	for _, t := range v.schema.IssueTypes {
		if t.Name == name {
			return
		}
		names = append(names, t.Name)
	}
	v.add(path, name, withHint("issue type does not exist on the organization", name, names), commands)
}

// checkDates validates the date fields against their declared source:
// project date fields, or organization issue fields when discovered.
func (v *validator) checkDates() {
	d := v.cfg.Capabilities.Dates
	if d == nil {
		return
	}
	names := map[string]string{"capabilities.dates.start": d.Start, "capabilities.dates.target": d.Target}
	for _, path := range []string{"capabilities.dates.start", "capabilities.dates.target"} {
		switch d.Source {
		case domain.DateSourceProject:
			v.field(path, names[path], domain.DataTypeDate, cmdsDates)
		case domain.DateSourceIssueFields:
			v.issueField(path, names[path])
		default:
			v.add("capabilities.dates.source", string(d.Source), "source must be issue_fields or project", cmdsDates)
			return
		}
	}
}

// issueField checks a name against the organization issue fields when
// they were discovered.
func (v *validator) issueField(path, name string) {
	if v.schema.IssueFields == nil || domain.Contains(v.schema.IssueFields, name) {
		return
	}
	v.add(path, name, withHint("issue field does not exist on the organization", name, v.schema.IssueFields), cmdsDates)
}

// checkLabels validates the triage and lab labels against the repository
// labels when they were discovered.
func (v *validator) checkLabels() {
	if v.schema.Labels == nil {
		return
	}
	c := v.cfg.Capabilities
	if c.Triage != nil {
		v.label("capabilities.triage.label", c.Triage.Label, cmdsTriage)
		if c.Triage.UrgentLabel != "" {
			v.label("capabilities.triage.urgent_label", c.Triage.UrgentLabel, cmdsTriage)
		}
	}
	if c.Lab != nil {
		v.label("capabilities.lab.label", c.Lab.Label, cmdsLab)
	}
}

func (v *validator) label(path, name string, commands []string) {
	names := make([]string, 0, len(v.schema.Labels))
	for _, l := range v.schema.Labels {
		if strings.EqualFold(l.Name, name) {
			return
		}
		names = append(names, l.Name)
	}
	v.add(path, name, withHint("label does not exist on the repository", name, names), commands)
}
