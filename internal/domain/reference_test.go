package domain_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

const (
	testOwner = "acme"
	testRepo  = "app"
	testURL   = "https://github.com/acme/app/issues/12"
)

var referenceCases = []struct {
	in   string
	want domain.Reference
}{
	{"#12", domain.Reference{Kind: domain.RefNumber, Raw: "#12", Number: 12}},
	{"12", domain.Reference{Kind: domain.RefNumber, Raw: "12", Number: 12}},
	{" #7 ", domain.Reference{Kind: domain.RefNumber, Raw: "#7", Number: 7}},
	{"acme/app#12", domain.Reference{Kind: domain.RefFull, Raw: "acme/app#12", Owner: testOwner, Repo: testRepo, Number: 12}},
	{testURL, domain.Reference{Kind: domain.RefURL, Raw: testURL, URL: testURL, Owner: testOwner, Repo: testRepo, Number: 12}},
	{"https://github.com/o/r.js/pull/3/", domain.Reference{Kind: domain.RefURL, Raw: "https://github.com/o/r.js/pull/3/", URL: "https://github.com/o/r.js/pull/3/", Owner: "o", Repo: "r.js", Number: 3}},
	{"I_kwDOAbCdEf5xYz12", domain.Reference{Kind: domain.RefNodeID, Raw: "I_kwDOAbCdEf5xYz12", NodeID: "I_kwDOAbCdEf5xYz12"}},
	{"PVTI_lADOBk7Llc4BmOZbzgHxyz0", domain.Reference{Kind: domain.RefNodeID, Raw: "PVTI_lADOBk7Llc4BmOZbzgHxyz0", NodeID: "PVTI_lADOBk7Llc4BmOZbzgHxyz0"}},
	{"MDU6SXNzdWU0NzU2OTM1NjA=", domain.Reference{Kind: domain.RefNodeID, Raw: "MDU6SXNzdWU0NzU2OTM1NjA=", NodeID: "MDU6SXNzdWU0NzU2OTM1NjA="}},
}

var invalidReferences = []string{
	"", "   ", "#0", "0", "owner/repo", "owner/repo#", "a/b#12x", "foo bar", "status",
	"I_kw", "http://github.com/o/r/issues/1", "https://github.com/o/r/commits/1",
	"https://github.com/o/r/issues/0", "https://github.com", "https:///o/r/issues/1",
}

func TestParseReference(t *testing.T) {
	for _, tc := range referenceCases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := domain.ParseReference(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseReferenceInvalid(t *testing.T) {
	for _, in := range invalidReferences {
		t.Run(in, func(t *testing.T) {
			_, err := domain.ParseReference(in)
			if !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
			if domain.CodeOf(err) != domain.ExitUsage {
				t.Fatalf("expected usage exit code, got %v", domain.CodeOf(err))
			}
		})
	}
}

var stringCases = map[string]string{
	"#12": "#12", "12": "#12", "acme/app#12": "acme/app#12",
	testURL: "acme/app#12", "I_kwDOAbCdEf5xYz12": "I_kwDOAbCdEf5xYz12",
}

func TestReferenceString(t *testing.T) {
	for in, want := range stringCases {
		ref, err := domain.ParseReference(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := ref.String(); got != want {
			t.Errorf("%s: String() = %q, want %q", in, got, want)
		}
	}
	if raw := (domain.Reference{Raw: "x"}).String(); raw != "x" {
		t.Errorf("unknown kind falls back to Raw, got %q", raw)
	}
}

func TestReferenceWithRepository(t *testing.T) {
	ref, _ := domain.ParseReference("#5")
	if !ref.NeedsRepository() {
		t.Fatal("number references need the repository")
	}
	full := ref.WithRepository(testOwner, testRepo)
	if full.Kind != domain.RefFull || full.Owner != testOwner || full.Repo != testRepo || full.Number != 5 {
		t.Fatalf("WithRepository = %+v", full)
	}
	if full.NeedsRepository() {
		t.Fatal("full references do not need the repository")
	}
	if again := full.WithRepository("x", "y"); again != full {
		t.Fatalf("WithRepository must not change a full reference: %+v", again)
	}
}

func FuzzParseReference(f *testing.F) {
	for _, tc := range referenceCases {
		f.Add(tc.in)
	}
	for _, in := range invalidReferences {
		f.Add(in)
	}
	f.Fuzz(func(t *testing.T, in string) {
		ref, err := domain.ParseReference(in)
		if err != nil {
			return
		}
		again, err := domain.ParseReference(ref.String())
		if err != nil {
			t.Fatalf("String() of %+v does not parse: %v", ref, err)
		}
		if again.Owner != ref.Owner || again.Repo != ref.Repo || again.Number != ref.Number || again.NodeID != ref.NodeID {
			t.Fatalf("round trip changed %+v into %+v", ref, again)
		}
	})
}
