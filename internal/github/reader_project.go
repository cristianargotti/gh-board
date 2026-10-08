package github

import (
	"context"
	"fmt"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// nodeProject is the type name of a board node.
const nodeProject = "ProjectV2"

// rawProjectNode is the ProjectByID answer: a node that may be a board.
type rawProjectNode struct {
	TypeName string `json:"__typename"`
	rawProject
}

// ProjectByID discovers a board by node id, which a resumed init knows
// from its journal when the copy's number was never printed, and caches
// the schema like DiscoverProject.
func (a *Adapter) ProjectByID(ctx context.Context, id string) (domain.Project, error) {
	if id == "" {
		return domain.Project{}, usage("project_by_id: a project id is required")
	}
	conn, err := a.connect()
	if err != nil {
		return domain.Project{}, err
	}
	var out struct {
		Node   *rawProjectNode `json:"node"`
		Viewer rawUser         `json:"viewer"`
	}
	if err := a.run(ctx, "project_by_id", map[string]any{varID: id}, &out); err != nil {
		return domain.Project{}, err
	}
	if out.Node == nil || out.Node.TypeName != nodeProject {
		return domain.Project{}, notFound("project_by_id", fmt.Sprintf("node %s is not a project", id))
	}
	p := toProject(&out.Node.rawProject, out.Viewer.Login, a.clock.Now())
	// A cache directory that cannot be written must not break a read.
	_ = a.writeCache(conn.host, p)
	return p, nil
}
