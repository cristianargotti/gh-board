package domain

import (
	"context"
	"time"
)

// ProjectReader is the read side of the GitHub adapter. Every method is a
// GraphQL query with variables; nothing here mutates.
type ProjectReader interface {
	// DiscoverProject reads the board schema of section 5.1: title, README,
	// fields with options and iterations, views, workflows, linked
	// repositories, item count and the viewer's role.
	DiscoverProject(ctx context.Context, ref ProjectRef) (Project, error)
	// ListItems returns one page of items with their field values. Filters
	// apply on the client after the page is read unless the adapter can
	// push them down; All reads every page.
	ListItems(ctx context.Context, project Project, opts ListOptions) (ItemPage, error)
	// GetItem resolves a reference to exactly one item, fresh from the API.
	// It returns ErrNotFound or ErrAmbiguous when the reference does not
	// resolve to exactly one project item.
	GetItem(ctx context.Context, project Project, ref Reference) (Item, error)
	// Viewer resolves the effective identity and the source of its token.
	Viewer(ctx context.Context) (Viewer, error)
	// ViewerPermission reports the viewer's permission on a repository.
	ViewerPermission(ctx context.Context, owner, repo string) (Permission, error)
	// RepositoryLabels lists the labels of a repository.
	RepositoryLabels(ctx context.Context, owner, repo string) ([]Label, error)
	// RepositoryMilestones lists the open milestones of a repository.
	RepositoryMilestones(ctx context.Context, owner, repo string) ([]Milestone, error)
	// IssueTypes lists the organization issue types available to owner.
	IssueTypes(ctx context.Context, owner string) ([]IssueType, error)
	// IssueComments reads the comments of an issue timeline, oldest first.
	IssueComments(ctx context.Context, issueID string) ([]Comment, error)
}

// ProjectWriter is the write side of the adapter: exactly the mutation
// allowlist of section 6.2, one typed method per mutation, nothing else.
// Methods marked "plans only" are reached only through apply.
type ProjectWriter interface {
	// CreateIssue runs createIssue.
	CreateIssue(ctx context.Context, in CreateIssueInput) (Issue, error)
	// UpdateIssue runs updateIssue for title, body and milestone only.
	UpdateIssue(ctx context.Context, in UpdateIssueInput) (Issue, error)
	// AddAssignees runs addAssigneesToAssignable.
	AddAssignees(ctx context.Context, assignableID string, userIDs []string) error
	// RemoveAssignees runs removeAssigneesFromAssignable.
	RemoveAssignees(ctx context.Context, assignableID string, userIDs []string) error
	// AddLabels runs addLabelsToLabelable.
	AddLabels(ctx context.Context, labelableID string, labelIDs []string) error
	// RemoveLabels runs removeLabelsFromLabelable.
	RemoveLabels(ctx context.Context, labelableID string, labelIDs []string) error
	// AddComment runs addComment and returns the created comment.
	AddComment(ctx context.Context, subjectID, body string) (Comment, error)
	// CloseIssue runs closeIssue.
	CloseIssue(ctx context.Context, issueID string) error
	// ReopenIssue runs reopenIssue.
	ReopenIssue(ctx context.Context, issueID string) error
	// AddSubIssue runs addSubIssue, linking child under parent.
	AddSubIssue(ctx context.Context, parentID, childID string) error
	// UpdateIssueType runs updateIssueIssueType.
	UpdateIssueType(ctx context.Context, issueID, issueTypeID string) error
	// SetIssueFieldValue runs setIssueFieldValue (organization issue field,
	// first value).
	SetIssueFieldValue(ctx context.Context, in IssueFieldValueInput) error
	// UpdateIssueFieldValue runs updateIssueFieldValue (existing value).
	UpdateIssueFieldValue(ctx context.Context, in IssueFieldValueInput) error
	// AddProjectItem runs addProjectV2ItemById and returns the item id.
	AddProjectItem(ctx context.Context, projectID, contentID string) (string, error)
	// UpdateItemFieldValue runs updateProjectV2ItemFieldValue.
	UpdateItemFieldValue(ctx context.Context, in ItemFieldValueInput) error
	// UnarchiveItem runs unarchiveProjectV2Item (restore).
	UnarchiveItem(ctx context.Context, projectID, itemID string) error
	// CreateStatusUpdate runs createProjectV2StatusUpdate (digest post).
	CreateStatusUpdate(ctx context.Context, in StatusUpdateInput) (string, error)
	// CreateProjectField runs createProjectV2Field, new fields only (plans only).
	CreateProjectField(ctx context.Context, in CreateFieldInput) (Field, error)
	// CreateLabel runs createLabel (plans only).
	CreateLabel(ctx context.Context, in CreateLabelInput) (Label, error)
	// CreateMilestone creates a milestone through REST POST (plans only).
	CreateMilestone(ctx context.Context, in CreateMilestoneInput) (Milestone, error)
	// CopyProject runs copyProjectV2 and returns the new project (plans only).
	CopyProject(ctx context.Context, in CopyProjectInput) (Project, error)
	// LinkProjectToRepository runs linkProjectV2ToRepository (plans only).
	LinkProjectToRepository(ctx context.Context, projectID, repositoryID string) error
}

// Clock abstracts time so that rules and plans are testable.
type Clock interface {
	// Now returns the current instant.
	Now() time.Time
}
