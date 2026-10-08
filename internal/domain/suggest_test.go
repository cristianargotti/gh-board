package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var statusOptions = []string{stBacklog, stReady, stActive, stTest, stDone}

var suggestCases = []struct {
	name  string
	input string
	want  string
	ok    bool
}{
	{"exact ignoring case", "done", stDone, true},
	{"typo", "IN PROGRES", stActive, true},
	{"transposition", "DNOE", stDone, true},
	{"substring", "VALIDATION", stTest, true},
	{"nothing close", "xyz", "", false},
	{"empty", "   ", "", false},
	{"closest wins", "READY TO DEX", stReady, true},
}

func TestSuggest(t *testing.T) {
	for _, tc := range suggestCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := domain.Suggest(tc.input, statusOptions)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("Suggest = %q, %v, want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

var distanceCases = map[string]int{"ab": 2, "abcdef": 2, "abcdefghi": 3, "abcdefghijkl": 4, "": 2}

func TestSuggestionDistance(t *testing.T) {
	for in, want := range distanceCases {
		if got := domain.SuggestionDistance(in); got != want {
			t.Errorf("SuggestionDistance(%q) = %d, want %d", in, got, want)
		}
	}
}

var levenshteinCases = []struct {
	a, b string
	want int
}{
	{"", "", 0}, {"", "abc", 3}, {"abc", "", 3}, {"kitten", "sitting", 3}, {"same", "same", 0}, {"épico", "epico", 1}, {"ab", "ba", 2},
}

func TestLevenshtein(t *testing.T) {
	for _, tc := range levenshteinCases {
		if got := domain.Levenshtein(tc.a, tc.b); got != tc.want {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNotFound(t *testing.T) {
	err := domain.NotFound("option", "DNOE", statusOptions)
	if !errors.Is(err, domain.ErrNotFound) || domain.CodeOf(err) != domain.ExitNotFound {
		t.Fatalf("NotFound = %v, code %v", err, domain.CodeOf(err))
	}
	if err.Error() != `option "DNOE" not found, did you mean "DONE"?: target not found` {
		t.Fatalf("message = %q", err.Error())
	}
	plain := domain.NotFound("label", "zzz", []string{"bug"})
	if plain.Error() != `label "zzz" not found: target not found` {
		t.Fatalf("message = %q", plain.Error())
	}
	same := domain.NotFound("option", "done", statusOptions)
	if strings.Contains(same.Error(), "did you mean") {
		t.Fatalf("a case-only difference is not a hint: %v", same)
	}
}

func TestResolveFieldAndOption(t *testing.T) {
	project := fixtureProject()
	field, err := domain.ResolveField(project, "status")
	if err != nil || field.ID != "F_status" {
		t.Fatalf("ResolveField = %+v, %v", field, err)
	}
	if _, err := domain.ResolveField(project, "Statuz"); err == nil || !strings.Contains(err.Error(), `did you mean "Status"`) {
		t.Fatalf("ResolveField error = %v", err)
	}
	option, err := domain.ResolveOption(field, "in progress")
	if err != nil || option.ID != "o3" {
		t.Fatalf("ResolveOption = %+v, %v", option, err)
	}
	if _, err := domain.ResolveOption(field, "DNOE"); err == nil || !strings.HasPrefix(err.Error(), `option of Status "DNOE" not found, did you mean "DONE"?`) {
		t.Fatalf("ResolveOption error = %v", err)
	}
	exact := domain.Field{Name: "F", Options: []domain.FieldOption{{ID: "a", Name: "done"}, {ID: "b", Name: "DONE"}}}
	if got, _ := domain.ResolveOption(exact, "DONE"); got.ID != "b" {
		t.Fatalf("an exact match wins over a case-insensitive one: %+v", got)
	}
}

func TestResolveIteration(t *testing.T) {
	sprint, _ := fixtureProject().FieldByName(fxSprint)
	it, err := domain.ResolveIteration(sprint, "sprint 6")
	if err != nil || it.ID != "i6" {
		t.Fatalf("ResolveIteration = %+v, %v", it, err)
	}
	if _, err := domain.ResolveIteration(sprint, "Sprint 7"); !errors.Is(err, domain.ErrNotFound) || !strings.Contains(err.Error(), "iteration of Sprint") {
		t.Fatalf("ResolveIteration error = %v", err)
	}
}

func TestResolveLabelsUsersMilestones(t *testing.T) {
	labels := []domain.Label{{ID: "L1", Name: "entrada"}, {ID: "L2", Name: "urgente"}}
	if l, err := domain.ResolveLabel(labels, "Entrada"); err != nil || l.ID != "L1" {
		t.Fatalf("ResolveLabel = %+v, %v", l, err)
	}
	if _, err := domain.ResolveLabel(labels, "urgent"); err == nil || !strings.Contains(err.Error(), `did you mean "urgente"`) {
		t.Fatalf("ResolveLabel error = %v", err)
	}
	users := []domain.User{{ID: "U1", Login: "alice"}, {ID: "U2", Login: "bob"}}
	if u, err := domain.ResolveUser(users, "BOB"); err != nil || u.ID != "U2" {
		t.Fatalf("ResolveUser = %+v, %v", u, err)
	}
	if _, err := domain.ResolveUser(users, "alise"); err == nil || !strings.Contains(err.Error(), `login "alise" not found, did you mean "alice"`) {
		t.Fatalf("ResolveUser error = %v", err)
	}
	milestones := []domain.Milestone{{ID: "M1", Title: "Q4 2026"}, {ID: "M2", Title: "Lançamento"}}
	if m, err := domain.ResolveMilestone(milestones, "q4 2026"); err != nil || m.ID != "M1" {
		t.Fatalf("ResolveMilestone = %+v, %v", m, err)
	}
	if _, err := domain.ResolveMilestone(milestones, "Lancamento"); err == nil || !strings.Contains(err.Error(), `did you mean "Lançamento"`) {
		t.Fatalf("ResolveMilestone error = %v", err)
	}
}
