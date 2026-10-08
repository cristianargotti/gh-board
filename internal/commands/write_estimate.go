package commands

import (
	"math"
	"strconv"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeEstimateCommand(deps *Deps) *cobra.Command {
	return writeCommand("estimate <ref[,ref...]> <days>", "Set the configured estimate in days", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeEstimate(it, args[1]) })
			})
		})
}

func (s *writeSession) writeEstimate(it domain.Item, value string) error {
	days, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(days) || math.IsInf(days, 0) || days < 0 {
		return domain.Errorf(domain.ExitUsage, "estimate must be a finite, nonnegative number of days")
	}
	task := s.cfg.Capabilities.Task
	if task != nil && task.MaxEstimateDays > 0 && it.Issue.Type != nil && strings.EqualFold(it.Issue.Type.Name, task.IssueType) && days > float64(task.MaxEstimateDays) {
		return domain.Errorf(domain.ExitPolicy, "task estimate exceeds %d days; split the task", task.MaxEstimateDays)
	}
	capability := s.cfg.Capabilities.Estimate
	if capability == nil {
		return writeUnavailable("estimate")
	}
	field, err := domain.ResolveField(s.project, capability.Field)
	if err != nil {
		return err
	}
	if field.DataType != domain.DataTypeNumber {
		return domain.Errorf(domain.ExitUsage, "estimate must be a number field")
	}
	return s.writeField(it, field, value)
}
