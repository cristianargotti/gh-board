package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// DraftInput is what init discovered. IssueTypes and IssueFields are not
// part of the project and come from the organization; nil means they were
// not discovered.
type DraftInput struct {
	Project    domain.Project
	Repository string
	// IssueTypes are listed as candidates for epic and task, never assigned.
	IssueTypes []domain.IssueType
	// IssueFields are the organization issue field names, the second
	// source of dates when the project has no date fields of those names.
	IssueFields []string
}

// Names section 5.4 allows init to map without a human decision.
const (
	draftStatusField = "Status"
	draftStartField  = "Start date"
	draftTargetField = "Target date"
)

// capabilityKeys lists every capability in the order of section 5.2, for
// the unavailable report.
var capabilityKeys = []string{capStatus, capEpic, capTask, capLane, capSprint, capEstimate, capDates, capBlocked, capTriage, capLab}

// Draft writes the board.yml draft of section 5.4 from the project alone.
func Draft(project domain.Project, repository string) ([]byte, error) {
	return DraftFrom(DraftInput{Project: project, Repository: repository})
}

// DraftFrom writes the board.yml draft of section 5.4: the field named
// exactly Status with its options in a comment, a single iteration field as
// sprint, the fields named exactly Start date and Target date as dates with
// the source detected from their kind, and the issue types as candidates.
// Everything else is absent and listed as unavailable in a trailing
// comment. The output decodes strictly.
func DraftFrom(in DraftInput) ([]byte, error) {
	if in.Project.Ref.IsZero() {
		return nil, fmt.Errorf("init needs a discovered project to draft board.yml: %w", domain.ErrUsage)
	}
	if in.Repository != "" {
		if _, _, err := domain.SplitRepository(in.Repository); err != nil {
			return nil, err
		}
	}
	var b strings.Builder
	writeHeader(&b, in)
	mapped := writeCapabilities(&b, in)
	writeCandidates(&b, in.IssueTypes)
	writeUnavailable(&b, mapped)
	return []byte(b.String()), nil
}

func writeHeader(b *strings.Builder, in DraftInput) {
	title := strings.TrimSpace(strings.ReplaceAll(in.Project.Title, "\n", " "))
	fmt.Fprintf(b, "# board.yml draft written by gh board init from %s", in.Project.Ref)
	if title != "" {
		fmt.Fprintf(b, " (%s)", title)
	}
	b.WriteString(".\n")
	b.WriteString("# Only unambiguous facts are mapped; review every line before the team relies on it.\n")
	b.WriteString("# Unknown keys, capability names, alert ids and metrics are rejected on load.\n")
	fmt.Fprintf(b, "version: %d\n", SupportedVersion)
	fmt.Fprintf(b, "project:\n  owner: %s\n  number: %d\n", scalar(in.Project.Ref.Owner), in.Project.Ref.Number)
	if in.Repository != "" {
		fmt.Fprintf(b, "repository: %s # where new issues are created and the default for \"#n\"\n", scalar(in.Repository))
		return
	}
	b.WriteString("# repository: owner/name # required for new and for the short form \"#n\"\n")
}

// writeCapabilities writes the mapped capabilities and returns their keys.
func writeCapabilities(b *strings.Builder, in DraftInput) []string {
	var blocks []string
	var mapped []string
	if block, ok := statusBlock(in.Project); ok {
		blocks, mapped = append(blocks, block), append(mapped, capStatus)
	}
	if block, ok := sprintBlock(in.Project); ok {
		blocks, mapped = append(blocks, block), append(mapped, capSprint)
	}
	if block, ok := datesBlock(in); ok {
		blocks, mapped = append(blocks, block), append(mapped, capDates)
	}
	if len(blocks) == 0 {
		b.WriteString("capabilities: {} # nothing could be mapped without a decision; see the notes below\n")
		return nil
	}
	b.WriteString("capabilities:\n")
	for _, block := range blocks {
		b.WriteString(block)
	}
	return mapped
}

// statusBlock maps the field named exactly Status when it is a single
// select, listing its options for the human to distribute.
func statusBlock(p domain.Project) (string, bool) {
	f, ok := p.FieldByName(draftStatusField)
	if !ok || f.DataType != domain.DataTypeSingleSelect {
		return "", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  status:\n    field: %s\n", scalar(f.Name))
	fmt.Fprintf(&b, "    # Options on the board: %s\n", strings.Join(f.OptionNames(), ", "))
	b.WriteString("    # Distribute every option among backlog, ready, active and done; moving to a done option closes the issue.\n")
	b.WriteString("    backlog: []\n    ready: []\n    active: []\n    done: []\n")
	return b.String(), true
}

// sprintBlock maps the iteration field when the board has exactly one.
func sprintBlock(p domain.Project) (string, bool) {
	var iterations []domain.Field
	for _, f := range p.Fields {
		if f.DataType == domain.DataTypeIteration {
			iterations = append(iterations, f)
		}
	}
	if len(iterations) != 1 {
		return "", false
	}
	return fmt.Sprintf("  sprint:\n    field: %s # the only iteration field on the board\n", scalar(iterations[0].Name)), true
}

// datesBlock maps Start date and Target date when both exist as project
// date fields (source project) or both as organization issue fields
// (source issue_fields). Project fields win when both kinds exist.
func datesBlock(in DraftInput) (string, bool) {
	source, ok := datesSource(in)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("  dates:\n    start: %s\n    target: %s\n    source: %s # detected from the field kind\n",
		scalar(draftStartField), scalar(draftTargetField), source), true
}

func datesSource(in DraftInput) (domain.DateSource, bool) {
	start, okStart := in.Project.FieldByName(draftStartField)
	target, okTarget := in.Project.FieldByName(draftTargetField)
	if okStart && okTarget && start.DataType == domain.DataTypeDate && target.DataType == domain.DataTypeDate {
		return domain.DateSourceProject, true
	}
	if domain.Contains(in.IssueFields, draftStartField) && domain.Contains(in.IssueFields, draftTargetField) {
		return domain.DateSourceIssueFields, true
	}
	return "", false
}

// writeCandidates lists the organization issue types without assigning
// them (section 5.4).
func writeCandidates(b *strings.Builder, types []domain.IssueType) {
	if len(types) == 0 {
		b.WriteString("# No issue types were discovered on the organization: epic and task stay unmapped.\n")
		return
	}
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.Name)
	}
	fmt.Fprintf(b, "# Issue types on the organization, candidates for epic.issue_type and task.issue_type: %s\n", strings.Join(names, ", "))
	b.WriteString("# epic: { issue_type: <type>, field: <single select field that names the epic> }\n")
	b.WriteString("# task: { issue_type: <type>, max_estimate_days: <days> }\n")
}

// writeUnavailable reports every capability the draft left absent.
func writeUnavailable(b *strings.Builder, mapped []string) {
	var missing []string
	for _, key := range capabilityKeys {
		if !domain.Contains(mapped, key) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(b, "# Unavailable until mapped by hand: %s.\n", strings.Join(missing, ", "))
	}
	b.WriteString("# Also absent: policy, tidy, alerts, digest, rituals. The reference board.yml in templates shows every key.\n")
}

// scalar renders a name as a YAML scalar, quoted only when YAML needs it.
func scalar(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	out, err := yaml.Marshal(s)
	if err != nil {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return strings.TrimSuffix(string(out), "\n")
}
