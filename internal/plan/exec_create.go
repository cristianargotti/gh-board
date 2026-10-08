package plan

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// addComment posts After on the target with the plan marker appended, and
// skips when a comment with that marker already exists.
func (e *executor) addComment(ctx context.Context, step domain.Step) (Outcome, error) {
	issueID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	marker := Marker(e.plan.ID, step.Index)
	comments, err := e.ports.Reader.IssueComments(ctx, issueID)
	if err != nil {
		return Outcome{}, err
	}
	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return skipped(c.ID, "comment already posted")
		}
	}
	comment, err := e.ports.Writer.AddComment(ctx, issueID, markedBody(step.After, marker))
	if err != nil {
		return Outcome{}, err
	}
	return created(comment.ID, "comment posted")
}

// createIssue creates the issue described by the CreateIssueInput JSON of
// After. GitHub cannot tell a duplicate, so only the journal guards it.
func (e *executor) createIssue(ctx context.Context, step domain.Step) (Outcome, error) {
	var in domain.CreateIssueInput
	if err := decodePayload(step, &in); err != nil {
		return Outcome{}, err
	}
	issue, err := e.ports.Writer.CreateIssue(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	return created(issue.NodeID, "created "+issue.Ref())
}

// createField creates the project field of the CreateFieldInput JSON of
// After, skipping a field the re-read project already has.
func (e *executor) createField(ctx context.Context, step domain.Step) (Outcome, error) {
	var in domain.CreateFieldInput
	if err := decodePayload(step, &in); err != nil {
		return Outcome{}, err
	}
	projectID, err := e.resolve(in.ProjectID)
	if err != nil {
		return Outcome{}, err
	}
	in.ProjectID = projectID
	if projectID == e.snap.Project.NodeID {
		if f, err := domain.ResolveField(e.snap.Project, in.Name); err == nil {
			return skipped(f.ID, "field already exists")
		}
	}
	field, err := e.ports.Writer.CreateProjectField(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	return created(field.ID, "field created")
}

// createLabel creates the label of the CreateLabelInput JSON of After in
// the repository of the target, skipping a label that already exists.
func (e *executor) createLabel(ctx context.Context, step domain.Step) (Outcome, error) {
	var in domain.CreateLabelInput
	if err := decodePayload(step, &in); err != nil {
		return Outcome{}, err
	}
	owner, repo := splitRepo(step.Target.Repository)
	labels, err := e.ports.Reader.RepositoryLabels(ctx, owner, repo)
	if err != nil {
		return Outcome{}, err
	}
	if l, err := domain.ResolveLabel(labels, in.Name); err == nil {
		return skipped(l.ID, "label already exists")
	}
	label, err := e.ports.Writer.CreateLabel(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	return created(label.ID, "label created")
}

// createMilestone creates the milestone of the CreateMilestoneInput JSON
// of After, skipping a title that already exists in the repository.
func (e *executor) createMilestone(ctx context.Context, step domain.Step) (Outcome, error) {
	var in domain.CreateMilestoneInput
	if err := decodePayload(step, &in); err != nil {
		return Outcome{}, err
	}
	milestones, err := e.ports.Reader.RepositoryMilestones(ctx, in.Owner, in.Repo)
	if err != nil {
		return Outcome{}, err
	}
	if m, err := domain.ResolveMilestone(milestones, in.Title); err == nil {
		return skipped(m.ID, "milestone already exists")
	}
	m, err := e.ports.Writer.CreateMilestone(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	return created(m.ID, "milestone created")
}

// copyProject copies the template of the CopyProjectInput JSON of After.
func (e *executor) copyProject(ctx context.Context, step domain.Step) (Outcome, error) {
	var in domain.CopyProjectInput
	if err := decodePayload(step, &in); err != nil {
		return Outcome{}, err
	}
	project, err := e.ports.Writer.CopyProject(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	return created(project.NodeID, "project "+project.Ref.String()+" created")
}

// linkRepository links the repository node of After to the project of
// the target, skipping a link the re-read project already has.
func (e *executor) linkRepository(ctx context.Context, step domain.Step) (Outcome, error) {
	projectID, err := e.target(step)
	if err != nil {
		return Outcome{}, err
	}
	repositoryID, err := e.resolve(step.After)
	if err != nil {
		return Outcome{}, err
	}
	if projectID == e.snap.Project.NodeID {
		for _, r := range e.snap.Project.Repositories {
			if r.NodeID == repositoryID {
				return skipped("", "repository already linked")
			}
		}
	}
	return finish(e.ports.Writer.LinkProjectToRepository(ctx, projectID, repositoryID))
}
