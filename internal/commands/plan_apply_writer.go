package commands

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

type planProjectResolver interface {
	ProjectByID(context.Context, string) (domain.Project, error)
}

type planApplyWriter struct {
	domain.ProjectWriter
	reader domain.ProjectReader
	state  string
	p      domain.Plan
	copied domain.Project
}

func (writer *planApplyWriter) planApplyPreflight(ctx context.Context) error {
	if writer.ProjectWriter == nil {
		return domain.Errorf(domain.ExitUsage, "a writer is required")
	}
	if err := writer.planProjectPermission(ctx); err != nil {
		return err
	}
	for _, step := range writer.p.Steps {
		if step.Operation == domain.OpCreateStatusUpdate {
			if _, ok := writer.reader.(plan.StatusUpdateLister); !ok {
				return domain.Errorf(domain.ExitAPI, "status update history is required for weekly idempotency")
			}
		}
		if err := writer.planStepPermission(ctx, step); err != nil {
			return err
		}
	}
	return nil
}

func (writer *planApplyWriter) CopyProject(ctx context.Context, input domain.CopyProjectInput) (domain.Project, error) {
	project, err := writer.ProjectWriter.CopyProject(ctx, input)
	if err == nil {
		writer.copied = project
	}
	return project, err
}

func (writer *planApplyWriter) LinkProjectToRepository(ctx context.Context, projectID, repositoryID string) error {
	if writer.planHasCopy() {
		id, err := writer.planCopiedID()
		if err != nil {
			return err
		}
		projectID = id
	}
	if projectID == "" {
		return domain.Errorf(domain.ExitApplyRefused, "project node id is unavailable for repository link")
	}
	return writer.ProjectWriter.LinkProjectToRepository(ctx, projectID, repositoryID)
}

func (writer *planApplyWriter) CreateStatusUpdate(ctx context.Context, input domain.StatusUpdateInput) (string, error) {
	reader, ok := writer.reader.(plan.StatusUpdateLister)
	if !ok {
		return "", domain.Errorf(domain.ExitAPI, "status update history is required for weekly idempotency")
	}
	if input.StartDate == nil {
		return "", domain.Errorf(domain.ExitApplyRefused, "digest step has no ISO week date")
	}
	marker := plan.WeekMarker(*input.StartDate)
	if !strings.Contains(input.Body, marker) {
		return "", domain.Errorf(domain.ExitApplyRefused, "digest step has no ISO week marker")
	}
	if writer.state == "" {
		return "", domain.Errorf(domain.ExitUsage, "a state directory is required for digest locking")
	}
	name := fmt.Sprintf("digest-%x.lock", sha256.Sum256([]byte(input.ProjectID+"|"+marker)))
	release, err := audit.Lock(filepath.Join(writer.state, plan.JournalDir, name))
	if err != nil {
		return "", domain.Errorf(domain.ExitApplyRefused, "digest lock unavailable: %v", err)
	}
	defer func() { _ = release() }()
	updates, err := reader.ListStatusUpdates(ctx, input.ProjectID)
	if err != nil {
		return "", planAPI(err)
	}
	for _, update := range updates {
		if strings.Contains(update.Body, marker) {
			return update.ID, nil
		}
	}
	return writer.ProjectWriter.CreateStatusUpdate(ctx, input)
}

func (writer *planApplyWriter) planHasCopy() bool {
	for _, step := range writer.p.Steps {
		if step.Operation == domain.OpCopyProject {
			return true
		}
	}
	return false
}

func (writer *planApplyWriter) planCopiedID() (string, error) {
	if writer.copied.NodeID != "" {
		return writer.copied.NodeID, nil
	}
	entries, err := plan.NewJournal(writer.state).Read(writer.p.ID)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Operation == domain.OpCopyProject && entry.Status == plan.StatusDone && entry.Created != "" {
			return entry.Created, nil
		}
	}
	return "", domain.Errorf(domain.ExitApplyRefused, "copied project node id is missing from the journal")
}

func (writer *planApplyWriter) planFinishInit(ctx context.Context) error {
	if writer.p.Command != planCommandInit {
		return nil
	}
	project, err := writer.planInitProject(ctx)
	if err != nil {
		return err
	}
	for _, step := range writer.p.Steps {
		if step.Operation != domain.OpLinkRepository {
			continue
		}
		var local planInitLocal
		if err := json.Unmarshal([]byte(step.Field), &local); err != nil {
			return domain.Errorf(domain.ExitApplyRefused, "invalid init local output metadata: %v", err)
		}
		if err := planInitWrite(local, project); err != nil {
			return err
		}
	}
	return nil
}

func (writer *planApplyWriter) planInitProject(ctx context.Context) (domain.Project, error) {
	if !writer.planHasCopy() {
		return writer.reader.DiscoverProject(ctx, writer.p.Project)
	}
	if !writer.copied.Ref.IsZero() {
		return writer.reader.DiscoverProject(ctx, writer.copied.Ref)
	}
	id, err := writer.planCopiedID()
	if err != nil {
		return domain.Project{}, err
	}
	resolver, ok := writer.reader.(planProjectResolver)
	if !ok {
		return domain.Project{}, domain.Errorf(domain.ExitAPI, "project node id discovery is required to resume init")
	}
	return resolver.ProjectByID(ctx, id)
}

func (writer *planApplyWriter) planProjectPermission(ctx context.Context) error {
	if writer.p.Project.IsZero() {
		return nil
	}
	project, err := writer.reader.DiscoverProject(ctx, writer.p.Project)
	if err != nil {
		return planAPI(err)
	}
	if project.ViewerRole != domain.RoleAdmin && project.ViewerRole != domain.RoleWriter {
		return domain.Errorf(domain.ExitPolicy, "project write permission is required")
	}
	return nil
}

func (writer *planApplyWriter) planStepPermission(ctx context.Context, step domain.Step) error {
	if step.Target.Repository == "" {
		return nil
	}
	owner, repo, err := domain.SplitRepository(step.Target.Repository)
	if err != nil {
		return err
	}
	permission, err := writer.reader.ViewerPermission(ctx, owner, repo)
	if err != nil {
		return planAPI(err)
	}
	return domain.CheckPermission(permission, step.Target.Repository)
}
