package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/cristianargotti/gh-board/internal/render"
)

// autoWatchReport is the --json form of one run.
type autoWatchReport struct {
	Project     domain.ProjectRef `json:"project"`
	GeneratedAt time.Time         `json:"generated_at"`
	Summary     string            `json:"summary"`
	New         int               `json:"new"`
	Escalated   int               `json:"escalated"`
	Resolved    int               `json:"resolved"`
	Unchanged   int               `json:"unchanged"`
	Notified    int               `json:"notified"`
	DryRun      bool              `json:"dry_run"`
	StatePath   string            `json:"state_path"`
	Pending     []domain.Alert    `json:"pending"`
}

// autoWatchOnce evaluates, diffs, notifies and writes alerts.json.
func autoWatchOnce(ctx context.Context, deps *Deps, loaded config.Loaded) error {
	project, err := autoRequireProject(loaded)
	if err != nil {
		return err
	}
	if deps.Reader == nil {
		return errors.New("watch needs the GitHub reader")
	}
	now := deps.Now()
	release, err := alerts.Lock(deps.Dirs.State, nil)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()
	board, items, err := autoReadBoard(ctx, deps.Reader, project)
	if err != nil {
		return err
	}
	statePath := alerts.StatePath(deps.Dirs.State)
	previous, err := alerts.ReadState(statePath)
	if err != nil {
		return fmt.Errorf("%w (remove the file to start over)", err)
	}
	current := autoEvaluateState(loaded.Config, board, items, now, previous)
	diff, err := autoDiffStates(previous, current)
	if err != nil {
		return err
	}
	report := autoWatchReport{
		Project: project, GeneratedAt: now, Summary: current.Summary, New: len(diff.New),
		Escalated: len(diff.Escalated), Resolved: len(diff.Resolved), Unchanged: diff.Unchanged,
		DryRun: deps.Flags.DryRun, StatePath: statePath, Pending: diff.Notifiable(),
	}
	if !deps.Flags.DryRun {
		if report.Notified, err = autoCommitWatch(ctx, deps, statePath, current, report.Pending, now); err != nil {
			return err
		}
	}
	return autoRender(deps, autoWatchDocument(report))
}

// autoCommitWatch is the side of a run that touches the world: the
// notifications, alerts.json and the retention pruning.
func autoCommitWatch(ctx context.Context, deps *Deps, statePath string, current domain.AlertState, pending []domain.Alert, now time.Time) (int, error) {
	sent, err := autoNotifyBudget(ctx, deps.Dirs.State, pending, now, deps.Out)
	if err != nil {
		return 0, err
	}
	if err := autoWriteState(statePath, current); err != nil {
		return sent, err
	}
	if _, err := alerts.Prune(deps.Dirs.State, now); err != nil {
		autoWarn(deps, "watch: prune the audit log: %v", err)
	}
	if _, err := plan.Prune(deps.Dirs.State, now.Add(-audit.Retention)); err != nil {
		autoWarn(deps, "watch: prune the plans: %v", err)
	}
	return sent, nil
}

// autoReadBoard discovers the project and reads every item with the
// summary selection, which is all the alert rules need.
func autoReadBoard(ctx context.Context, reader domain.ProjectReader, ref domain.ProjectRef) (domain.Project, []domain.Item, error) {
	project, err := reader.DiscoverProject(ctx, ref)
	if err != nil {
		return domain.Project{}, nil, err
	}
	var items []domain.Item
	opts := domain.ListOptions{All: true, Selection: domain.SelectionSummary}
	for {
		page, err := reader.ListItems(ctx, project, opts)
		if err != nil {
			return domain.Project{}, nil, err
		}
		items = append(items, page.Items...)
		if !page.HasNext || page.NextCursor == "" || page.NextCursor == opts.Cursor {
			return project, items, nil
		}
		opts.Cursor = page.NextCursor
	}
}

// autoEvaluateState runs the one alert function of section 9 and builds
// the alerts.json contract with the summary the mod shows.
func autoEvaluateState(cfg *domain.Config, project domain.Project, items []domain.Item, now time.Time, previous domain.AlertState) domain.AlertState {
	found := domain.EvaluateAlerts(domain.AlertInput{Config: cfg, Project: project, Items: items, Now: now, Previous: &previous})
	if found == nil {
		found = []domain.Alert{}
	}
	sprint := domain.SprintSummaryOf(cfg, project, items, now)
	return domain.AlertState{
		GeneratedAt: now,
		Project:     cfg.Project,
		Summary:     domain.AlertSummary(sprint, found),
		Alerts:      found,
	}
}

// autoWatchDocument renders a run for the terminal and for --json.
func autoWatchDocument(r autoWatchReport) *render.Document {
	doc := render.NewDocument("Watch")
	s := doc.AddSection("Alerts")
	s.AddKeyValue("Project", r.Project.String())
	s.AddKeyValue("Summary", r.Summary)
	s.AddKeyValue("New", strconv.Itoa(r.New))
	s.AddKeyValue("Escalated", strconv.Itoa(r.Escalated))
	s.AddKeyValue("Resolved", strconv.Itoa(r.Resolved))
	s.AddKeyValue("Unchanged", strconv.Itoa(r.Unchanged))
	s.AddKeyValue("Notified", strconv.Itoa(r.Notified))
	if len(r.Pending) > 0 {
		table := s.SetTable("Rule", "Severity", "Item", "Message")
		for _, a := range r.Pending {
			table.AddRow(a.RuleID, string(a.Severity), autoAlertItem(a.Item), a.Message)
		}
	}
	if r.DryRun {
		s.AddNote("dry run: nothing was notified and alerts.json was not written")
	} else {
		s.AddNote("state written to " + r.StatePath)
	}
	doc.Data = r
	return doc
}

// autoBoardItem names a board-level alert in the table.
const autoBoardItem = "board"

// autoAlertItem names the item of an alert, or the board itself.
func autoAlertItem(ref domain.ItemRef) string {
	if ref.Repository == "" || ref.Number == 0 {
		return autoBoardItem
	}
	return fmt.Sprintf("%s#%d", ref.Repository, ref.Number)
}
