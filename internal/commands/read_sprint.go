package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// Sprint aliases accepted by sprint and by list --sprint.
const (
	readSprintCurrent = "current"
	readSprintNext    = "next"
)

// readSprintPayload is the --json form of sprint current|next.
type readSprintPayload struct {
	GeneratedAt string                `json:"generated_at"`
	Which       string                `json:"which"`
	Available   bool                  `json:"available"`
	Reason      string                `json:"reason,omitempty"`
	Sprint      *domain.SprintSummary `json:"sprint"`
	Items       []domain.ItemSummary  `json:"items"`
}

func readSprintCurrentCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     readSprintCurrent,
		Short:   "The sprint in progress with its items",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readSprint(cmd.Context(), deps, readSprintCurrent) },
	}
}

func readSprintNextCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     readSprintNext,
		Short:   "The sprint scheduled after the current one",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readSprint(cmd.Context(), deps, readSprintNext) },
	}
}

func readSprint(ctx context.Context, deps *Deps, which string) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	day := domain.Today(s.now, domain.LocationOf(s.cfg))
	field, iteration, reason := readResolveIteration(s.cfg, s.project, day, which)
	payload := readSprintPayload{GeneratedAt: readStamp(s.now), Which: which, Reason: reason, Items: []domain.ItemSummary{}}
	doc := render.NewDocument("Sprint " + which)
	if reason != "" {
		doc.AddSection("").AddNote(reason).AddKeyValue(readKeyGenerated, payload.GeneratedAt)
		doc.Data = payload
		return readRender(deps, doc)
	}
	items, err := s.summaries(ctx)
	if err != nil {
		return err
	}
	inSprint := make([]domain.Item, 0)
	for _, it := range items {
		if !it.Archived && it.Text(field.Name) == iteration.Title {
			inSprint = append(inSprint, it)
		}
	}
	payload.Available = true
	payload.Sprint = readIterationSummary(s.cfg, iteration, inSprint, day)
	payload.Items = readDelimitSummaries(domain.SummarizeItems(s.cfg, inSprint))
	doc.Title = iteration.Title
	readSprintSection(doc.AddSection(""), payload.Sprint, s.cfg)
	readSummaryTable(doc.AddSection(readKeyItems), payload.Items, readColumnsOf(s.cfg))
	doc.AddSection("").AddKeyValue(readKeyGenerated, payload.GeneratedAt)
	doc.Data = payload
	return readRender(deps, doc)
}

// readResolveIteration finds the iteration named by which on the calendar
// date day: current is the one in progress, next the first one starting
// after it (or after the day when none is in progress), anything else an
// iteration title. The reason is non-empty when nothing can be shown.
func readResolveIteration(cfg *domain.Config, project domain.Project, day time.Time, which string) (domain.Field, domain.Iteration, string) {
	caps := domain.CapabilitiesOf(cfg)
	if caps.Sprint == nil || caps.Sprint.Field == "" {
		return domain.Field{}, domain.Iteration{}, "sprint " + readUnavailable
	}
	field, ok := project.FieldByName(caps.Sprint.Field)
	if !ok {
		return domain.Field{}, domain.Iteration{}, fmt.Sprintf("sprint field %q not found on the board", caps.Sprint.Field)
	}
	switch strings.ToLower(which) {
	case readSprintCurrent:
		if it, ok := field.IterationAt(day); ok {
			return field, it, ""
		}
		return field, domain.Iteration{}, "no sprint in progress"
	case readSprintNext:
		if it, ok := readNextIteration(field, day); ok {
			return field, it, ""
		}
		return field, domain.Iteration{}, "no sprint scheduled after the current one"
	default:
		it, err := domain.ResolveIteration(field, which)
		if err != nil {
			return field, domain.Iteration{}, err.Error()
		}
		return field, it, ""
	}
}

// readNextIteration picks the earliest iteration that starts at or after
// the end of the current one, or after the day when none is in progress.
func readNextIteration(field domain.Field, day time.Time) (domain.Iteration, bool) {
	from := day
	if current, ok := field.IterationAt(day); ok {
		from = current.End()
	}
	var best domain.Iteration
	found := false
	for _, it := range field.Iterations {
		if it.Start.Before(from) {
			continue
		}
		if !found || it.Start.Before(best.Start) {
			best, found = it, true
		}
	}
	return best, found
}

// readIterationSummary counts the open and done items of one iteration.
func readIterationSummary(cfg *domain.Config, it domain.Iteration, items []domain.Item, day time.Time) *domain.SprintSummary {
	sum := domain.IterationSummary(it, day)
	for _, item := range items {
		if domain.IsDone(cfg, item) {
			sum.Done++
		} else {
			sum.Open++
		}
	}
	return sum
}
