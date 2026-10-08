package domain

import (
	"fmt"
	"strings"
)

// Suggest finds the candidate the input most likely meant. An exact match
// ignoring case returns the canonical spelling; otherwise the closest
// candidate within SuggestionDistance of the input, or a candidate that
// starts with or contains the input.
func Suggest(input string, candidates []string) (string, bool) {
	needle := strings.ToLower(strings.TrimSpace(input))
	if needle == "" {
		return "", false
	}
	for _, c := range candidates {
		if strings.ToLower(c) == needle {
			return c, true
		}
	}
	best, bestDistance := "", -1
	for _, c := range candidates {
		d := Levenshtein(needle, strings.ToLower(c))
		if d <= SuggestionDistance(needle) && (bestDistance < 0 || d < bestDistance) {
			best, bestDistance = c, d
		}
	}
	if bestDistance >= 0 {
		return best, true
	}
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c), needle) {
			return c, true
		}
	}
	return "", false
}

// SuggestionDistance is the largest edit distance accepted for an input:
// a third of its length, at least two edits.
func SuggestionDistance(input string) int {
	if d := len([]rune(input)) / 3; d > 2 {
		return d
	}
	return 2
}

// Levenshtein counts the single-rune edits that turn a into b.
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	previous := make([]int, len(rb)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		current := make([]int, len(rb)+1)
		current[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(rb)]
}

// NotFound builds the ErrNotFound error of a failed lookup with a "did you
// mean" hint when one candidate is close.
func NotFound(kind, input string, candidates []string) error {
	if hint, ok := Suggest(input, candidates); ok && !strings.EqualFold(hint, input) {
		return fmt.Errorf("%s %q not found, did you mean %q?: %w", kind, input, hint, ErrNotFound)
	}
	return fmt.Errorf("%s %q not found: %w", kind, input, ErrNotFound)
}

// resolve returns the index of the candidate matching the input, exactly
// or ignoring case, or the not found error.
func resolve(kind, input string, candidates []string) (int, error) {
	for i, c := range candidates {
		if c == input {
			return i, nil
		}
	}
	for i, c := range candidates {
		if strings.EqualFold(c, input) {
			return i, nil
		}
	}
	return -1, NotFound(kind, input, candidates)
}

// ResolveField finds a project field by name, ignoring case.
func ResolveField(project Project, name string) (Field, error) {
	i, err := resolve("field", name, project.FieldNames())
	if err != nil {
		return Field{}, err
	}
	return project.Fields[i], nil
}

// ResolveOption finds a single select option by name, ignoring case.
func ResolveOption(field Field, name string) (FieldOption, error) {
	i, err := resolve("option of "+field.Name, name, field.OptionNames())
	if err != nil {
		return FieldOption{}, err
	}
	return field.Options[i], nil
}

// ResolveIteration finds an iteration by title, ignoring case.
func ResolveIteration(field Field, title string) (Iteration, error) {
	titles := make([]string, 0, len(field.Iterations))
	for _, it := range field.Iterations {
		titles = append(titles, it.Title)
	}
	i, err := resolve("iteration of "+field.Name, title, titles)
	if err != nil {
		return Iteration{}, err
	}
	return field.Iterations[i], nil
}

// ResolveLabel finds a repository label by name, ignoring case.
func ResolveLabel(labels []Label, name string) (Label, error) {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	i, err := resolve("label", name, names)
	if err != nil {
		return Label{}, err
	}
	return labels[i], nil
}

// ResolveUser finds a user by login, ignoring case.
func ResolveUser(users []User, login string) (User, error) {
	logins := make([]string, 0, len(users))
	for _, u := range users {
		logins = append(logins, u.Login)
	}
	i, err := resolve("login", login, logins)
	if err != nil {
		return User{}, err
	}
	return users[i], nil
}

// ResolveMilestone finds a milestone by title, ignoring case.
func ResolveMilestone(milestones []Milestone, title string) (Milestone, error) {
	titles := make([]string, 0, len(milestones))
	for _, m := range milestones {
		titles = append(titles, m.Title)
	}
	i, err := resolve("milestone", title, titles)
	if err != nil {
		return Milestone{}, err
	}
	return milestones[i], nil
}
