package domain

// Config is board.yml decoded strictly (section 5.2): unknown keys, unknown
// capability names and unknown alert ids are errors. It carries no
// permission or policy-weakening key; security policy is compiled in.
type Config struct {
	Version      int          `yaml:"version"      json:"version"`
	Project      ProjectRef   `yaml:"project"      json:"project"`
	Repository   string       `yaml:"repository"   json:"repository"`
	Language     string       `yaml:"language"     json:"language"`
	Timezone     string       `yaml:"timezone"     json:"timezone"`
	Capabilities Capabilities `yaml:"capabilities" json:"capabilities"`
	Policy       Policy       `yaml:"policy"       json:"policy"`
	Tidy         Tidy         `yaml:"tidy"         json:"tidy"`
	Alerts       AlertRules   `yaml:"alerts"       json:"alerts"`
	Digest       Digest       `yaml:"digest"       json:"digest"`
	Rituals      []Ritual     `yaml:"rituals"      json:"rituals"`
	Template     *Template    `yaml:"template,omitempty" json:"template,omitempty"`
}

// Capabilities declares which roles the board's fields play. A nil
// capability is unavailable and the commands that need it say so.
type Capabilities struct {
	Status   *StatusCapability `yaml:"status,omitempty"   json:"status,omitempty"`
	Epic     *EpicCapability   `yaml:"epic,omitempty"     json:"epic,omitempty"`
	Task     *TaskCapability   `yaml:"task,omitempty"     json:"task,omitempty"`
	Lane     *FieldCapability  `yaml:"lane,omitempty"     json:"lane,omitempty"`
	Sprint   *FieldCapability  `yaml:"sprint,omitempty"   json:"sprint,omitempty"`
	Estimate *FieldCapability  `yaml:"estimate,omitempty" json:"estimate,omitempty"`
	Dates    *DatesCapability  `yaml:"dates,omitempty"    json:"dates,omitempty"`
	Blocked  *FieldCapability  `yaml:"blocked,omitempty"  json:"blocked,omitempty"`
	Triage   *TriageCapability `yaml:"triage,omitempty"   json:"triage,omitempty"`
	Lab      *LabCapability    `yaml:"lab,omitempty"      json:"lab,omitempty"`
}

// StatusCapability maps the status field options to the four classes.
type StatusCapability struct {
	Field   string   `yaml:"field"   json:"field"`
	Backlog []string `yaml:"backlog" json:"backlog"`
	Ready   []string `yaml:"ready"   json:"ready"`
	Active  []string `yaml:"active"  json:"active"`
	Done    []string `yaml:"done"    json:"done"`
}

// StatusClass is the class of a status option.
type StatusClass string

// Status classes. StatusUnknown marks an option the file does not map.
const (
	StatusBacklog StatusClass = "backlog"
	StatusReady   StatusClass = "ready"
	StatusActive  StatusClass = "active"
	StatusDone    StatusClass = "done"
	StatusUnknown StatusClass = "unknown"
)

// Classify returns the class of a status option name.
func (s *StatusCapability) Classify(option string) StatusClass {
	if s == nil {
		return StatusUnknown
	}
	for class, names := range map[StatusClass][]string{
		StatusBacklog: s.Backlog, StatusReady: s.Ready, StatusActive: s.Active, StatusDone: s.Done,
	} {
		for _, n := range names {
			if n == option {
				return class
			}
		}
	}
	return StatusUnknown
}

// IsDone reports whether moving to the option closes the issue downstream,
// which makes the move a plan (principle 3).
func (s *StatusCapability) IsDone(option string) bool {
	return s.Classify(option) == StatusDone
}

// EpicCapability names the issue type and the field that mark epics.
type EpicCapability struct {
	IssueType string `yaml:"issue_type" json:"issue_type"`
	Field     string `yaml:"field"      json:"field"`
}

// TaskCapability names the task issue type and the estimate ceiling.
type TaskCapability struct {
	IssueType       string `yaml:"issue_type"        json:"issue_type"`
	MaxEstimateDays int    `yaml:"max_estimate_days" json:"max_estimate_days"`
}

// FieldCapability binds a role to one field by name.
type FieldCapability struct {
	Field string `yaml:"field" json:"field"`
}

// DateSource says where start and target dates live.
type DateSource string

// Date sources of section 5.2.
const (
	DateSourceIssueFields DateSource = "issue_fields"
	DateSourceProject     DateSource = "project"
)

// DatesCapability names the start and target date fields and their source.
type DatesCapability struct {
	Start  string     `yaml:"start"  json:"start"`
	Target string     `yaml:"target" json:"target"`
	Source DateSource `yaml:"source" json:"source"`
}

// TriageCapability describes the entry queue and its SLA.
type TriageCapability struct {
	Label           string `yaml:"label"             json:"label"`
	DecisionField   string `yaml:"decision_field"    json:"decision_field"`
	SLABusinessDays int    `yaml:"sla_business_days" json:"sla_business_days"`
	UrgentLabel     string `yaml:"urgent_label"      json:"urgent_label"`
}

// LabCapability describes the lab experiments gate and result fields.
type LabCapability struct {
	Label       string `yaml:"label"        json:"label"`
	GateField   string `yaml:"gate_field"   json:"gate_field"`
	ResultField string `yaml:"result_field" json:"result_field"`
}

// Policy is the team process: allowed transitions, WIP limits and the bulk
// threshold above which a direct write becomes a plan.
type Policy struct {
	Transitions   map[string][]string `yaml:"transitions"    json:"transitions"`
	WIP           map[string]int      `yaml:"wip"            json:"wip"`
	BulkThreshold int                 `yaml:"bulk_threshold" json:"bulk_threshold"`
}

// Tidy declares the routine steps tidy may plan.
type Tidy struct {
	InheritFromParent  []string `yaml:"inherit_from_parent"   json:"inherit_from_parent"`
	SprintFromTarget   bool     `yaml:"sprint_from_target"    json:"sprint_from_target"`
	StampStartOnActive bool     `yaml:"stamp_start_on_active" json:"stamp_start_on_active"`
	EpicFollowsTasks   bool     `yaml:"epic_follows_tasks"    json:"epic_follows_tasks"`
}

// AlertRules maps an alert id to its options; an empty mapping enables the
// rule with defaults. Unknown ids are rejected by config validation.
type AlertRules map[string]AlertRule

// AlertRule holds the options of one alert rule.
type AlertRule struct {
	Days int `yaml:"days,omitempty" json:"days,omitempty"`
}

// Digest configures the weekly digest.
type Digest struct {
	Weekday string   `yaml:"weekday" json:"weekday"`
	Metrics []string `yaml:"metrics" json:"metrics"`
	Members []string `yaml:"members" json:"members"`
}

// Ritual is a recurring meeting and the sections it reads.
type Ritual struct {
	Name  string   `yaml:"name"  json:"name"`
	When  string   `yaml:"when"  json:"when"`
	Reads []string `yaml:"reads" json:"reads"`
}

// Known identifiers the configuration may reference.
var (
	// AlertRuleIDs are the alert ids of section 9, in evaluation order.
	AlertRuleIDs = []string{
		AlertOverdue, AlertBlocked, AlertTriageSLA, AlertWIPExceeded,
		AlertSprintEnding, AlertStaleActive, AlertEpicWithoutDates,
	}
	// DigestMetrics are the metric names digest can compute.
	DigestMetrics = []string{"first_response", "autonomy", "run_share"}
	// TidyInheritable are the roles tidy may inherit from a parent epic.
	TidyInheritable = []string{"lane", "epic"}
)

// Contains reports whether the list holds the value.
func Contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
