package commands

import (
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

const (
	writeSprintCurrent = "current"
	writeSprintNext    = "next"
)

func writeSprintCommand(deps *Deps) *cobra.Command {
	return writeCommand("set <ref[,ref...]> <current|next|title>", "Set the configured sprint iteration", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeSprint(it, args[1]) })
			})
		})
}

func (s *writeSession) writeSprint(it domain.Item, title string) error {
	capability := s.cfg.Capabilities.Sprint
	if capability == nil {
		return writeUnavailable("sprint")
	}
	field, err := domain.ResolveField(s.project, capability.Field)
	if err != nil {
		return err
	}
	if field.DataType != domain.DataTypeIteration {
		return domain.Errorf(domain.ExitUsage, "sprint must be an iteration field")
	}
	iteration, err := writeIteration(field, title, domain.Today(s.deps.Now(), domain.LocationOf(s.cfg)))
	if err != nil {
		return err
	}
	return s.writeField(it, field, iteration.Title)
}

// writeIteration resolves current, next or a title on the calendar date.
func writeIteration(field domain.Field, title string, day time.Time) (domain.Iteration, error) {
	if title != writeSprintCurrent && title != writeSprintNext {
		return domain.ResolveIteration(field, title)
	}
	var selected domain.Iteration
	for _, it := range field.Iterations {
		if it.Completed {
			continue
		}
		if title == writeSprintCurrent && it.Contains(day) {
			return it, nil
		}
		if title == writeSprintNext && it.Start.After(day) && (selected.ID == "" || it.Start.Before(selected.Start)) {
			selected = it
		}
	}
	if selected.ID == "" {
		return selected, domain.Errorf(domain.ExitNotFound, "no %s sprint iteration", title)
	}
	return selected, nil
}
