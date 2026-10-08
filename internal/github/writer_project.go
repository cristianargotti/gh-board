package github

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Status update states of ProjectV2StatusUpdateStatus; the digest flag
// spells them in lower case.
var statusUpdateStates = []string{"INACTIVE", "ON_TRACK", "AT_RISK", "OFF_TRACK", "COMPLETE"}

// optionColor is the color of every option the kit creates; GitHub
// requires one and the team recolors in the UI.
const optionColor = "GRAY"

// AddProjectItem runs addProjectV2ItemById and returns the item id.
func (a *Adapter) AddProjectItem(ctx context.Context, projectID, contentID string) (string, error) {
	if projectID == "" || contentID == "" {
		return "", usage("add_project_item: project and content ids are required")
	}
	var out struct {
		AddProjectV2ItemByID struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	if err := a.run(ctx, "add_project_item", map[string]any{varProjectID: projectID, "contentId": contentID}, &out); err != nil {
		return "", err
	}
	return out.AddProjectV2ItemByID.Item.ID, nil
}

// UnarchiveItem runs unarchiveProjectV2Item (restore).
func (a *Adapter) UnarchiveItem(ctx context.Context, projectID, itemID string) error {
	if projectID == "" || itemID == "" {
		return usage("unarchive_item: project and item ids are required")
	}
	var out struct{}
	return a.run(ctx, "unarchive_item", map[string]any{varProjectID: projectID, "itemId": itemID}, &out)
}

// CreateStatusUpdate runs createProjectV2StatusUpdate (digest post) and
// returns the status update id.
func (a *Adapter) CreateStatusUpdate(ctx context.Context, in domain.StatusUpdateInput) (string, error) {
	if in.ProjectID == "" {
		return "", usage("create_status_update: a project id is required")
	}
	vars := map[string]any{varProjectID: in.ProjectID}
	setString(vars, varBody, in.Body)
	if in.Status != "" {
		status := strings.ToUpper(in.Status)
		if !domain.Contains(statusUpdateStates, status) {
			return "", usage("create_status_update: status %q is not one of %s", in.Status, strings.Join(statusUpdateStates, ", "))
		}
		vars["status"] = status
	}
	if in.StartDate != nil {
		vars["startDate"] = in.StartDate.Format(domain.DateLayout)
	}
	if in.TargetDate != nil {
		vars["targetDate"] = in.TargetDate.Format(domain.DateLayout)
	}
	var out struct {
		CreateProjectV2StatusUpdate struct {
			StatusUpdate struct {
				ID string `json:"id"`
			} `json:"statusUpdate"`
		} `json:"createProjectV2StatusUpdate"`
	}
	if err := a.run(ctx, "create_status_update", vars, &out); err != nil {
		return "", err
	}
	return out.CreateProjectV2StatusUpdate.StatusUpdate.ID, nil
}

// CreateProjectField runs createProjectV2Field, new fields only (plans
// only). Single select options get the default color and no description.
func (a *Adapter) CreateProjectField(ctx context.Context, in domain.CreateFieldInput) (domain.Field, error) {
	if in.ProjectID == "" || in.Name == "" || in.DataType == "" {
		return domain.Field{}, usage("create_project_field: project id, name and data type are required")
	}
	vars := map[string]any{varProjectID: in.ProjectID, varName: in.Name, "dataType": string(in.DataType)}
	if len(in.SingleSelectOptions) > 0 {
		options := make([]map[string]any, 0, len(in.SingleSelectOptions))
		for _, name := range in.SingleSelectOptions {
			options = append(options, map[string]any{varName: name, "color": optionColor, "description": ""})
		}
		vars["singleSelectOptions"] = options
	}
	var out struct {
		CreateProjectV2Field struct {
			ProjectV2Field rawField `json:"projectV2Field"`
		} `json:"createProjectV2Field"`
	}
	if err := a.run(ctx, "create_project_field", vars, &out); err != nil {
		return domain.Field{}, err
	}
	return toField(out.CreateProjectV2Field.ProjectV2Field), nil
}

// CreateLabel runs createLabel (plans only).
func (a *Adapter) CreateLabel(ctx context.Context, in domain.CreateLabelInput) (domain.Label, error) {
	if in.RepositoryID == "" || in.Name == "" || in.Color == "" {
		return domain.Label{}, usage("create_label: repository id, name and color are required")
	}
	vars := map[string]any{varRepositoryID: in.RepositoryID, varName: in.Name, "color": in.Color}
	setString(vars, "description", in.Description)
	var out struct {
		CreateLabel struct {
			Label rawLabel `json:"label"`
		} `json:"createLabel"`
	}
	if err := a.run(ctx, "create_label", vars, &out); err != nil {
		return domain.Label{}, err
	}
	return toLabel(out.CreateLabel.Label), nil
}

// CopyProject runs copyProjectV2 and returns the new project (plans only).
// A copy into a user account lands in the viewer's own account, so the
// owner login is the viewer's and the role comes out ADMIN.
func (a *Adapter) CopyProject(ctx context.Context, in domain.CopyProjectInput) (domain.Project, error) {
	if in.SourceProjectID == "" || in.OwnerID == "" || in.Title == "" {
		return domain.Project{}, usage("copy_project: source project id, owner id and title are required")
	}
	vars := map[string]any{
		varProjectID:         in.SourceProjectID,
		"ownerId":            in.OwnerID,
		varTitle:             in.Title,
		"includeDraftIssues": in.IncludeDraftIssues,
	}
	var out struct {
		CopyProjectV2 struct {
			ProjectV2 *rawProject `json:"projectV2"`
		} `json:"copyProjectV2"`
	}
	if err := a.run(ctx, "copy_project", vars, &out); err != nil {
		return domain.Project{}, err
	}
	raw := out.CopyProjectV2.ProjectV2
	if raw == nil {
		return domain.Project{}, notFound("copy_project", "GitHub returned no project")
	}
	return toProject(raw, raw.Owner.Login, a.clock.Now()), nil
}

// LinkProjectToRepository runs linkProjectV2ToRepository (plans only).
func (a *Adapter) LinkProjectToRepository(ctx context.Context, projectID, repositoryID string) error {
	if projectID == "" || repositoryID == "" {
		return usage("link_project_to_repository: project and repository ids are required")
	}
	var out struct{}
	vars := map[string]any{varProjectID: projectID, varRepositoryID: repositoryID}
	return a.run(ctx, "link_project_to_repository", vars, &out)
}
