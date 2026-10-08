package github_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestIssueFields(t *testing.T) {
	r, a := newReplay(t, "acme")
	ctx := context.Background()
	fields, err := a.IssueFields(ctx, "acme")
	if err != nil || len(fields) != 4 {
		t.Fatalf("fields: %v, %+v", err, fields)
	}
	priority := fields[1]
	if priority.Name != "Priority" || priority.DataType != domain.DataTypeSingleSelect || len(priority.Options) != 4 || priority.Options[0].Name != "Urgent" {
		t.Fatalf("priority = %+v", priority)
	}
	if start := fields[2]; start.Name != "Start date" || start.DataType != domain.DataTypeDate || start.ID != "IFD_e7jmu-_8WZ" {
		t.Fatalf("start date = %+v", start)
	}
	none, err := a.IssueFields(ctx, "membro1")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("a user owner has no issue fields and no error: %v, %v", err, none)
	}
	before := r.count()
	if _, err := a.IssueFields(ctx, ""); !errors.Is(err, domain.ErrUsage) || r.count() != before {
		t.Fatalf("an empty owner is refused before any request: %v", err)
	}
}

var issueFieldIDCases = []struct {
	name, owner, field, want string
	err                      error
	hint                     string
}{
	{"exact name", "acme", "Target date", "IFD_QPgBnZsICL", nil, ""},
	{"case differs", "acme", "target date", "IFD_QPgBnZsICL", nil, ""},
	{"close name", "acme", "Target dates", "", domain.ErrNotFound, "Target date"},
	{"unknown name", "acme", "Budget", "", domain.ErrNotFound, ""},
	{"user owner", "membro1", "Target date", "", domain.ErrNotFound, ""},
	{"empty name", "acme", "", "", domain.ErrUsage, ""},
}

func TestIssueFieldID(t *testing.T) {
	for _, tc := range issueFieldIDCases {
		t.Run(tc.name, func(t *testing.T) {
			_, a := newReplay(t, "acme")
			got, err := a.IssueFieldID(context.Background(), tc.owner, tc.field)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("id = %q, %v; want %q, %v", got, err, tc.want, tc.err)
			}
			if tc.hint != "" && !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("error %q lacks the hint %q", err, tc.hint)
			}
		})
	}
}
