package config

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// FormSubstitutions drives RenderForm. Values maps a reference field or
// label name to the team's name; IssueTypes maps the reference issue types
// (epic and task) to the team's; KnownTypes lists the issue types of the
// target organization, so a form whose type is not among them loses its
// type line; BoardURL replaces the board link of config.yml when set.
type FormSubstitutions struct {
	Values     map[string]string
	IssueTypes map[string]string
	KnownTypes []string
	BoardURL   string
}

var (
	quotedItem = regexp.MustCompile(`"([^"]*)"`)
	projectURL = regexp.MustCompile(`^(\s*url:\s*)https://github\.com/(?:orgs|users)/[^/\s]+/projects/\d+\s*$`)
)

// FormSubstitutionsFor derives the substitutions from the reference
// board.yml the forms were written against and the team's board.yml: the
// pairs templates/README.md lists. A capability the team did not declare
// keeps the reference text, following principle 6.
func FormSubstitutionsFor(reference, team *domain.Config, project domain.Project, types []domain.IssueType) FormSubstitutions {
	s := FormSubstitutions{Values: formPairs(reference, team), IssueTypes: formTypePairs(reference, team), KnownTypes: []string{}, BoardURL: project.URL}
	for _, t := range types {
		s.KnownTypes = append(s.KnownTypes, t.Name)
	}
	return s
}

func formPairs(reference, team *domain.Config) map[string]string {
	ref, tm := domain.CapabilitiesOf(reference), domain.CapabilitiesOf(team)
	pairs := map[string]string{}
	add := func(from, to string) {
		if from != "" && to != "" && from != to {
			pairs[from] = to
		}
	}
	for _, f := range [][2]*domain.FieldCapability{{ref.Lane, tm.Lane}, {ref.Estimate, tm.Estimate}} {
		if f[0] != nil && f[1] != nil {
			add(f[0].Field, f[1].Field)
		}
	}
	if ref.Epic != nil && tm.Epic != nil {
		add(ref.Epic.Field, tm.Epic.Field)
	}
	if ref.Dates != nil && tm.Dates != nil {
		add(ref.Dates.Start, tm.Dates.Start)
		add(ref.Dates.Target, tm.Dates.Target)
	}
	if ref.Triage != nil && tm.Triage != nil {
		add(ref.Triage.Label, tm.Triage.Label)
		add(ref.Triage.UrgentLabel, tm.Triage.UrgentLabel)
		add(ref.Triage.DecisionField, tm.Triage.DecisionField)
	}
	if ref.Lab != nil && tm.Lab != nil {
		add(ref.Lab.Label, tm.Lab.Label)
		add(ref.Lab.GateField, tm.Lab.GateField)
		add(ref.Lab.ResultField, tm.Lab.ResultField)
	}
	return pairs
}

func formTypePairs(reference, team *domain.Config) map[string]string {
	ref, tm := domain.CapabilitiesOf(reference), domain.CapabilitiesOf(team)
	pairs := map[string]string{}
	if ref.Epic != nil && tm.Epic != nil && ref.Epic.IssueType != "" && tm.Epic.IssueType != "" {
		pairs[ref.Epic.IssueType] = tm.Epic.IssueType
	}
	if ref.Task != nil && tm.Task != nil && ref.Task.IssueType != "" && tm.Task.IssueType != "" {
		pairs[ref.Task.IssueType] = tm.Task.IssueType
	}
	return pairs
}

// RenderForm rewrites one embedded issue form (or the config.yml of the
// forms) for the team and refuses a result that no longer parses as YAML,
// which a team name with a quote or a colon could cause.
func RenderForm(name string, data []byte, s FormSubstitutions) ([]byte, error) {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		rendered, keep := renderFormLine(line, s)
		if keep {
			out = append(out, rendered)
		}
	}
	text := strings.Join(out, "\n")
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || len(doc) == 0 {
		return nil, usage("form %s: the rendered YAML is invalid (%v); check the names in board.yml", name, oneLine(errOrEmpty(err)))
	}
	return []byte(text), nil
}

func errOrEmpty(err error) error {
	if err == nil {
		return errEmptyForm
	}
	return err
}

// errEmptyForm reports a form that rendered to nothing.
var errEmptyForm = domain.Errorf(domain.ExitUsage, "empty document")

// renderFormLine handles the three line kinds the forms use for names: the
// top-level issue type, the top-level label list and the board link; every
// other line receives the whole-word substitutions.
func renderFormLine(line string, s FormSubstitutions) (string, bool) {
	switch {
	case strings.HasPrefix(line, "type: "):
		name := strings.TrimSpace(strings.TrimPrefix(line, "type: "))
		if mapped, ok := s.IssueTypes[name]; ok {
			name = mapped
		}
		if !domain.Contains(s.KnownTypes, name) {
			return "", false
		}
		return "type: " + name, true
	case strings.HasPrefix(line, "labels: "):
		return quotedItem.ReplaceAllStringFunc(line, func(item string) string {
			inner := strings.Trim(item, `"`)
			if to, ok := s.Values[inner]; ok {
				return `"` + to + `"`
			}
			return item
		}), true
	case s.BoardURL != "" && projectURL.MatchString(line):
		return projectURL.FindStringSubmatch(line)[1] + s.BoardURL, true
	default:
		return replaceWords(line, s.Values), true
	}
}

// replaceWords substitutes every reference name that stands as a whole
// word, longest names first so that a name inside another is not cut.
func replaceWords(line string, values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j]) || (len(keys[i]) == len(keys[j]) && keys[i] < keys[j])
	})
	for _, from := range keys {
		line = replaceWord(line, from, values[from])
	}
	return line
}

func replaceWord(line, from, to string) string {
	var b strings.Builder
	for {
		i := strings.Index(line, from)
		if i < 0 {
			break
		}
		end := i + len(from)
		if !boundary(line, i, end) {
			b.WriteString(line[:end])
			line = line[end:]
			continue
		}
		b.WriteString(line[:i])
		b.WriteString(to)
		line = line[end:]
	}
	b.WriteString(line)
	return b.String()
}

// boundary reports whether the match at [start, end) stands as a whole
// word: no letter or digit touches it on either side.
func boundary(s string, start, end int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:start])
	after, _ := utf8.DecodeRuneInString(s[end:])
	return !wordRune(before) && !wordRune(after)
}

func wordRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.IsDigit(r))
}
