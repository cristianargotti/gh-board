package config

import "testing"

var suggestCases = []struct {
	name  string
	in    string
	known []string
	want  string
}{
	{"same letters another case", "status", []string{"Status", "Sprint"}, `"Status"`},
	{"surrounding spaces", " Status ", []string{"Status"}, `"Status"`},
	{"one edit away", "Frentes", []string{"Frente", "Sprint"}, `"Frente"`},
	{"two edits away", "Epic", []string{"Épico"}, `"Épico"`},
	{"too far", "Roadmap", []string{"Frente", "Sprint"}, ""},
	{"short names never guess", "QA", []string{"QB", "ABC"}, ""},
	{"nothing known", "Frente", nil, ""},
}

func TestSuggest(t *testing.T) {
	for _, tc := range suggestCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := suggest(tc.in, tc.known); got != tc.want {
				t.Fatalf("suggest(%q) = %q, want %q", tc.in, got, tc.want)
			}
			reason := withHint("missing", tc.in, tc.known)
			if (tc.want != "") != (reason != "missing") {
				t.Fatalf("withHint = %q", reason)
			}
		})
	}
}

var distanceCases = []struct {
	a, b string
	want int
}{
	{"", "", 0},
	{"abc", "", 3},
	{"kitten", "sitting", 3},
	{"Épico", "Epico", 1},
	{"same", "same", 0},
}

func TestDistance(t *testing.T) {
	for _, tc := range distanceCases {
		if got := distance(tc.a, tc.b); got != tc.want {
			t.Fatalf("distance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
