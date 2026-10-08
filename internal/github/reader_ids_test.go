package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Sanitized identities of the recordings used by the id lookups.
const (
	issue111ID     = "I_kud9Ny7kjDmOS1mJGdwQGO"
	fieldsIssueID  = "I_L6YAgk2eOGLqwgrX2JwpPl"
	acmeRepoID     = "R_UkssVNXKn-"
	missingLogin   = "membro132"
	organizationID = "MDEyOk9yZ2FuaXphdGlvbjI3MzQ4MzM="
)

func TestIssueByID(t *testing.T) {
	r, a := newReplay(t, "acme", "sandbox")
	ctx := context.Background()
	issue, err := a.IssueByID(ctx, issue111ID)
	if err != nil || issue.NodeID != issue111ID || issue.Number != 111 || issue.Ref() != "acme/app#111" {
		t.Fatalf("issue: %v, %+v", err, issue)
	}
	if issue.Title == "" || issue.State != domain.IssueClosed || len(issue.Labels) == 0 {
		t.Fatalf("issue fields = %+v", issue)
	}
	sandboxIssue, err := a.IssueByID(ctx, sandboxIssueID)
	if err != nil || sandboxIssue.Number != 1 || sandboxIssue.Owner != "membro1" {
		t.Fatalf("sandbox issue: %v, %+v", err, sandboxIssue)
	}
	if _, err := a.IssueByID(ctx, acmeProjectID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a project node is not an issue: %v", err)
	}
	before := r.count()
	if _, err := a.IssueByID(ctx, ""); !errors.Is(err, domain.ErrUsage) || r.count() != before {
		t.Fatalf("an empty id is refused before any request: %v", err)
	}
}

var loginCases = []struct {
	name, kind, login, want string
	err                     error
}{
	{"user by login", "user", "membro1", "U_2JRxF9gBtX", nil},
	{"user missing", "user", missingLogin, "", domain.ErrNotFound},
	{"user empty", "user", "", "", domain.ErrUsage},
	{"organization owner", "owner", "acme", organizationID, nil},
	{"user owner", "owner", "membro1", "U_2JRxF9gBtX", nil},
	{"owner missing", "owner", missingLogin, "", domain.ErrNotFound},
	{"owner empty", "owner", "", "", domain.ErrUsage},
}

func TestLoginIDs(t *testing.T) {
	for _, tc := range loginCases {
		t.Run(tc.name, func(t *testing.T) {
			_, a := newReplay(t, "acme")
			lookup := a.UserID
			if tc.kind == "owner" {
				lookup = a.OwnerID
			}
			got, err := lookup(context.Background(), tc.login)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("id = %q, %v; want %q, %v", got, err, tc.want, tc.err)
			}
		})
	}
}

var repositoryIDCases = []struct {
	name, owner, repo, want string
	err                     error
}{
	{"team repository", "acme", "app", acmeRepoID, nil},
	{"sandbox repository", "membro1", "gh-board-sandbox", sandboxRepoID, nil},
	{"missing repository", "acme", "repo-that-does-not-exist", "", domain.ErrNotFound},
	{"empty name", "acme", "", "", domain.ErrUsage},
}

func TestRepositoryID(t *testing.T) {
	for _, tc := range repositoryIDCases {
		t.Run(tc.name, func(t *testing.T) {
			_, a := newReplay(t, "acme", "sandbox")
			got, err := a.RepositoryID(context.Background(), tc.owner, tc.repo)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("id = %q, %v; want %q, %v", got, err, tc.want, tc.err)
			}
		})
	}
}
