package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

var emptyID = ""

// usageCalls are writes the adapter refuses before any request.
var usageCalls = map[string]func(context.Context, *github.Adapter) error{
	"create without title": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CreateIssue(ctx, domain.CreateIssueInput{RepositoryID: "R_x"})
		return err
	},
	"update nothing": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.UpdateIssue(ctx, domain.UpdateIssueInput{IssueID: "I_x"})
		return err
	},
	"clear milestone": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.UpdateIssue(ctx, domain.UpdateIssueInput{IssueID: "I_x", MilestoneID: &emptyID})
		return err
	},
	"assign nobody":       func(ctx context.Context, a *github.Adapter) error { return a.AddAssignees(ctx, "I_x", nil) },
	"label nothing":       func(ctx context.Context, a *github.Adapter) error { return a.RemoveLabels(ctx, "", []string{"LA_x"}) },
	"close nothing":       func(ctx context.Context, a *github.Adapter) error { return a.CloseIssue(ctx, "") },
	"link without parent": func(ctx context.Context, a *github.Adapter) error { return a.AddSubIssue(ctx, "", "I_x") },
	"clear type":          func(ctx context.Context, a *github.Adapter) error { return a.UpdateIssueType(ctx, "I_x", "") },
	"empty comment": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.AddComment(ctx, "I_x", "")
		return err
	},
	"clear issue field": func(ctx context.Context, a *github.Adapter) error {
		return a.SetIssueFieldValue(ctx, domain.IssueFieldValueInput{IssueID: "I_x", FieldID: "IFD_x"})
	},
	"item value without ids": func(ctx context.Context, a *github.Adapter) error {
		return a.UpdateItemFieldValue(ctx, domain.ItemFieldValueInput{ProjectID: "P"})
	},
	"add item without ids": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.AddProjectItem(ctx, "", "I_x")
		return err
	},
	"unarchive without ids": func(ctx context.Context, a *github.Adapter) error { return a.UnarchiveItem(ctx, "P", "") },
	"status update no board": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CreateStatusUpdate(ctx, domain.StatusUpdateInput{})
		return err
	},
	"bad status": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CreateStatusUpdate(ctx, domain.StatusUpdateInput{ProjectID: "P", Status: "late"})
		return err
	},
	"field without type": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CreateProjectField(ctx, domain.CreateFieldInput{ProjectID: "P", Name: "x"})
		return err
	},
	"label without color": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CreateLabel(ctx, domain.CreateLabelInput{RepositoryID: "R", Name: "x"})
		return err
	},
	"milestone without repo": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CreateMilestone(ctx, domain.CreateMilestoneInput{Title: "x"})
		return err
	},
	"copy without owner": func(ctx context.Context, a *github.Adapter) error {
		_, err := a.CopyProject(ctx, domain.CopyProjectInput{SourceProjectID: "P", Title: "x"})
		return err
	},
	"link without repo": func(ctx context.Context, a *github.Adapter) error { return a.LinkProjectToRepository(ctx, "P", "") },
}

func TestWriteUsageErrors(t *testing.T) {
	r, a := newReplay(t, "sandbox-writes")
	ctx := context.Background()
	for name, call := range usageCalls {
		t.Run(name, func(t *testing.T) {
			if err := call(ctx, a); !errors.Is(err, domain.ErrUsage) || domain.CodeOf(err) != domain.ExitUsage {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
		})
	}
	if r.count() != 0 {
		t.Fatalf("usage errors must be refused before any request, got %d", r.count())
	}
}
