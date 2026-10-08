package plan

import (
	"context"
	"errors"
	"fmt"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Snapshot is what apply re-read before the first write: the project
// schema and every pending target by node id.
type Snapshot struct {
	Project domain.Project
	Items   map[string]domain.Item
}

// Reread discovers the project and reads every target the pending steps
// touch, fresh from the API. A target that is not on the board is left
// out of Items and reported as a drift later; a target an earlier step
// creates cannot be read yet. A plan without a project (init copying a
// template) skips discovery.
func Reread(ctx context.Context, reader TargetReader, p domain.Plan, steps []domain.Step) (Snapshot, error) {
	return RereadCreated(ctx, reader, p, steps, nil)
}

// RereadCreated is Reread for a resumed plan: created lists the issue ids
// earlier attempts produced, so an issue GitHub does not find on the board
// yet (its add_project_item step still pends) is read as a bare issue
// through IssueReader instead of being left out as a drift.
func RereadCreated(ctx context.Context, reader TargetReader, p domain.Plan, steps []domain.Step, created map[string]bool) (Snapshot, error) {
	snap := Snapshot{Items: make(map[string]domain.Item)}
	if !p.Project.IsZero() {
		project, err := reader.DiscoverProject(ctx, p.Project)
		if err != nil {
			return snap, fmt.Errorf("re-read project %s: %w", p.Project, err)
		}
		snap.Project = project
	}
	seen := make(map[string]bool)
	for _, s := range steps {
		id := s.Target.NodeID
		if id == "" || isRef(id) || seen[id] || !needsItem(s.Operation) {
			continue
		}
		seen[id] = true
		item, err := reader.GetItem(ctx, snap.Project, domain.Reference{Kind: domain.RefNodeID, Raw: id, NodeID: id})
		if errors.Is(err, domain.ErrNotFound) && created[id] {
			item, err = rereadCreated(ctx, reader, id)
		}
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return snap, fmt.Errorf("re-read %s: %w", Label(s.Target), err)
		}
		snap.Items[id] = item
	}
	return snap, nil
}

// rereadCreated reads a created issue off the board; a reader without the
// port reports it not found, which keeps the drift behavior of before.
func rereadCreated(ctx context.Context, reader TargetReader, id string) (domain.Item, error) {
	issues, ok := reader.(IssueReader)
	if !ok {
		return domain.Item{}, fmt.Errorf("%s: %w", id, domain.ErrNotFound)
	}
	issue, err := issues.IssueByID(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	return domain.Item{Issue: issue, Values: map[string]domain.FieldValue{}}, nil
}
