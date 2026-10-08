package plan

import "github.com/cristianargotti/gh-board/internal/domain"

// Journal bindings must be concrete before the snapshot and drift checks;
// only targets that will be created in this attempt may remain references.
func bindTarget(step domain.Step, done map[int]JournalEntry) (domain.Target, error) {
	target := step.Target
	var err error
	target.NodeID, err = bindReference(target.NodeID, step.Index, done)
	if err != nil {
		return target, err
	}
	target.ProjectItemID, err = bindReference(target.ProjectItemID, step.Index, done)
	return target, err
}

func bindReference(id string, index int, done map[int]JournalEntry) (string, error) {
	if !isRef(id) {
		return id, nil
	}
	n, ok := parseRef(id)
	if !ok || n >= index {
		return "", refuse("step %d reference %q must name an earlier step", index, id)
	}
	entry, ok := done[n]
	if !ok {
		return id, nil
	}
	if entry.Created == "" || isRef(entry.Created) {
		return "", refuse("step %d reference %q has no created node in the journal", index, id)
	}
	return entry.Created, nil
}
