package plan

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// handler executes one operation.
type handler func(e *executor, ctx context.Context, step domain.Step) (Outcome, error)

// handlers maps every plan operation to exactly one allowlisted mutation
// (section 6.2); an operation outside this map cannot be applied.
var handlers = map[domain.Operation]handler{
	domain.OpCloseIssue:         (*executor).closeIssue,
	domain.OpReopenIssue:        (*executor).reopenIssue,
	domain.OpUpdateIssue:        (*executor).updateIssue,
	domain.OpSetIssueType:       (*executor).setIssueType,
	domain.OpAddSubIssue:        (*executor).addSubIssue,
	domain.OpUnarchiveItem:      (*executor).unarchiveItem,
	domain.OpAddProjectItem:     (*executor).addProjectItem,
	domain.OpSetFieldValue:      (*executor).setFieldValue,
	domain.OpSetIssueFieldValue: (*executor).setIssueFieldValue,
	domain.OpAddAssignees:       (*executor).addAssignees,
	domain.OpRemoveAssignees:    (*executor).removeAssignees,
	domain.OpAddLabels:          (*executor).addLabels,
	domain.OpRemoveLabels:       (*executor).removeLabels,
	domain.OpAddComment:         (*executor).addComment,
	domain.OpCreateIssue:        (*executor).createIssue,
	domain.OpCreateStatusUpdate: (*executor).createStatusUpdate,
	domain.OpCreateField:        (*executor).createField,
	domain.OpCreateLabel:        (*executor).createLabel,
	domain.OpCreateMilestone:    (*executor).createMilestone,
	domain.OpCopyProject:        (*executor).copyProject,
	domain.OpLinkRepository:     (*executor).linkRepository,
}

// KnownOperation reports whether apply can execute the operation.
func KnownOperation(op domain.Operation) bool {
	_, ok := handlers[op]
	return ok
}

// executor runs steps against the ports with what apply re-read.
type executor struct {
	ports   Ports
	plan    domain.Plan
	snap    Snapshot
	viewer  domain.Viewer
	now     time.Time
	created map[int]string
}

// NewExecutor builds the executor apply uses. created maps the index of
// every step already done to the node it created, so a resumed plan can
// follow step references; the executor adds what it creates.
func NewExecutor(ports Ports, p domain.Plan, snap Snapshot, viewer domain.Viewer, now time.Time, created map[int]string) Executor {
	if created == nil {
		created = make(map[int]string)
	}
	return &executor{ports: ports, plan: p, snap: snap, viewer: viewer, now: now, created: created}
}

// Execute runs one step and records what it created.
func (e *executor) Execute(ctx context.Context, step domain.Step) (Outcome, error) {
	h, ok := handlers[step.Operation]
	if !ok {
		return Outcome{}, fmt.Errorf("step %d: unknown operation %q: %w", step.Index, step.Operation, domain.ErrUsage)
	}
	out, err := h(e, ctx, step)
	if err != nil {
		return Outcome{}, fmt.Errorf("step %d (%s) failed: %w", step.Index, step.Operation, err)
	}
	if out.NodeID != "" {
		e.created[step.Index] = out.NodeID
	}
	return out, nil
}

// resolve follows a step reference to the node an earlier step created
// and returns any other id unchanged.
func (e *executor) resolve(id string) (string, error) {
	n, ok := parseRef(id)
	if !ok {
		return id, nil
	}
	created, ok := e.created[n]
	if !ok || created == "" {
		return "", fmt.Errorf("reference %q names a step that created nothing: %w", id, domain.ErrUsage)
	}
	return created, nil
}

// target resolves the node id of the step, which must not be empty.
func (e *executor) target(step domain.Step) (string, error) {
	id, err := e.resolve(step.Target.NodeID)
	if err == nil && id == "" {
		err = fmt.Errorf("step %d (%s) has no target: %w", step.Index, step.Operation, domain.ErrUsage)
	}
	return id, err
}

// item returns the re-read item of the step, when it was readable.
func (e *executor) item(step domain.Step) (domain.Item, bool) {
	it, ok := e.snap.Items[step.Target.NodeID]
	return it, ok
}

// projectID is the project the plan writes to.
func (e *executor) projectID() string {
	if e.plan.ProjectID != "" {
		return e.plan.ProjectID
	}
	return e.snap.Project.NodeID
}

// finish turns a mutation result into an outcome.
func finish(err error) (Outcome, error) {
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Status: StatusDone}, nil
}

// created reports a node the step created.
func created(nodeID, note string) (Outcome, error) {
	return Outcome{Status: StatusDone, NodeID: nodeID, Note: note}, nil
}

// skipped reports a step whose effect is already in place.
func skipped(nodeID, note string) (Outcome, error) {
	return Outcome{Status: StatusSkipped, NodeID: nodeID, Note: note}, nil
}
