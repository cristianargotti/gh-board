package plan

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// TargetReader is the narrow read port apply uses to re-read every target
// before the first write. domain.ProjectReader satisfies it.
type TargetReader interface {
	DiscoverProject(ctx context.Context, ref domain.ProjectRef) (domain.Project, error)
	GetItem(ctx context.Context, project domain.Project, ref domain.Reference) (domain.Item, error)
}

// UserResolver resolves a login to a user node id. Plans carry assignees
// by login; the executor asks the reader for the id when the login is not
// the viewer. The adapter gains the method at integration; until then such
// a step fails with ErrNotImplemented.
type UserResolver interface {
	UserID(ctx context.Context, login string) (string, error)
}

// IssueFieldResolver resolves an organization issue field by name for an
// issue that has no value in it yet. Same integration note as UserResolver.
type IssueFieldResolver interface {
	IssueFieldID(ctx context.Context, owner, name string) (string, error)
}

// StatusUpdate is one project status update as the lister reports it; the
// value type lives in domain so the GitHub adapter can return it without
// importing this package.
type StatusUpdate = domain.StatusUpdate

// IssueReader reads an issue that is not on the board yet: the issue a
// journaled create step produced while its add_project_item step still
// pends. A resumed apply re-reads such a target through it.
type IssueReader interface {
	IssueByID(ctx context.Context, id string) (domain.Issue, error)
}

// StatusUpdateLister lists the status updates of a project so digest post
// can skip a week that already has one. Without history the executor
// refuses to post because the journal cannot cover an uncertain API result.
type StatusUpdateLister interface {
	ListStatusUpdates(ctx context.Context, projectID string) ([]StatusUpdate, error)
}

// Executor runs one step against GitHub. Apply builds one with
// NewExecutor; the interface lets tests and future tiers wire another.
type Executor interface {
	Execute(ctx context.Context, step domain.Step) (Outcome, error)
}

// Outcome is what executing one step produced.
type Outcome struct {
	// Status is StatusDone or StatusSkipped.
	Status string
	// NodeID is the node the step created or found already in place.
	NodeID string
	// Note explains a skip or names what was created, for the output.
	Note string
}
