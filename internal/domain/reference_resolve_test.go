package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var splitRepositoryCases = []struct {
	in    string
	owner string
	name  string
	ok    bool
}{
	{"acme/team-docs", "acme", "team-docs", true},
	{" acme/team-docs ", "acme", "team-docs", true},
	{"acme", "", "", false},
	{"/team-docs", "", "", false},
	{"acme/", "", "", false},
	{"acme/team/docs", "", "", false},
	{"acme/team docs", "", "", false},
	{"acme/team#docs", "", "", false},
	{"acme/team.docs", "acme", "team.docs", true},
	{"acme/team_docs", "acme", "team_docs", true},
	{"0/!", "", "", false},
	{"acme_org/docs", "", "", false},
	{"acme-/docs", "", "", false},
	{"-acme/docs", "", "", false},
	{"", "", "", false},
}

func TestSplitRepository(t *testing.T) {
	for _, tc := range splitRepositoryCases {
		t.Run(tc.in, func(t *testing.T) {
			owner, name, err := domain.SplitRepository(tc.in)
			if (err == nil) != tc.ok || owner != tc.owner || name != tc.name {
				t.Fatalf("SplitRepository = %q, %q, %v", owner, name, err)
			}
			if err != nil && !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
		})
	}
}

var resolveReferenceCases = []struct {
	name       string
	in         string
	repository string
	want       string
	err        string
}{
	{"short form with repository", "#5", "acme/team-docs", "acme/team-docs#5", ""},
	{"bare number with repository", "5", "acme/team-docs", "acme/team-docs#5", ""},
	{"short form without repository", "#5", "", "", `reference "#5" needs repository in board.yml; use owner/repo#5 or the issue URL`},
	{"short form with blank repository", "5", "   ", "", "needs repository in board.yml"},
	{"short form with malformed repository", "#5", "acme", "", `repository "acme": expected owner/name`},
	{"full form ignores the repository", "other/repo#9", "acme/team-docs", "other/repo#9", ""},
	{"url ignores the repository", "https://github.com/other/repo/issues/9", "", "other/repo#9", ""},
	{"node id passes through", "I_kwDOAbCdEf5xYz12", "", "I_kwDOAbCdEf5xYz12", ""},
}

func TestResolveReference(t *testing.T) {
	for _, tc := range resolveReferenceCases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := domain.ParseReference(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			got, err := domain.ResolveReference(ref, tc.repository)
			if tc.err != "" {
				assertUsageError(t, err, tc.err)
				return
			}
			if err != nil || got.String() != tc.want || got.NeedsRepository() {
				t.Fatalf("ResolveReference = %+v, %v, want %q", got, err, tc.want)
			}
		})
	}
}

// assertUsageError expects a usage error (exit code 1) whose message
// carries the fragment.
func assertUsageError(t *testing.T, err error, fragment string) {
	t.Helper()
	if !errors.Is(err, domain.ErrUsage) || domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("error = %v, want a usage error", err)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error = %v, want %q", err, fragment)
	}
}

func FuzzResolveReference(f *testing.F) {
	for _, tc := range resolveReferenceCases {
		f.Add(tc.in, tc.repository)
	}
	f.Add("#1", "a/b")
	f.Add("12", "x/y#z")
	f.Fuzz(func(t *testing.T, in, repository string) {
		ref, err := domain.ParseReference(in)
		if err != nil {
			return
		}
		resolved, err := domain.ResolveReference(ref, repository)
		if err != nil {
			if !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("unexpected error kind: %v", err)
			}
			return
		}
		if resolved.NeedsRepository() {
			t.Fatalf("a resolved reference still needs the repository: %+v", resolved)
		}
		if _, err := domain.ParseReference(resolved.String()); err != nil {
			t.Fatalf("String() of %+v does not parse: %v", resolved, err)
		}
	})
}
