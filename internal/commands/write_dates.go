package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

type writeIssueFieldResolver interface {
	IssueFieldID(context.Context, string, string) (string, error)
}

func writeDatesCommand(deps *Deps) *cobra.Command {
	var start, target string
	cmd := writeCommand("dates <ref[,ref...]>", "Set configured start and target dates", cobra.ExactArgs(1),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				if start == "" && target == "" {
					return domain.Errorf(domain.ExitUsage, "provide --start or --target")
				}
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeDates(it, start, target) })
			})
		})
	cmd.Flags().StringVar(&start, "start", "", "start date, YYYY-MM-DD")
	cmd.Flags().StringVar(&target, "target", "", "target date, YYYY-MM-DD")
	return cmd
}

func (s *writeSession) writeDates(it domain.Item, start, target string) error {
	capability := s.cfg.Capabilities.Dates
	if capability == nil {
		return writeUnavailable("dates")
	}
	if capability.Source != domain.DateSourceProject && capability.Source != domain.DateSourceIssueFields {
		return domain.Errorf(domain.ExitUsage, "dates.source must be project or issue_fields")
	}
	if err := writeDateOrder(it, capability, start, target); err != nil {
		return err
	}
	if capability.Source == domain.DateSourceIssueFields {
		_, _ = fmt.Fprintln(s.cmd.ErrOrStderr(), "Warning: organization issue fields are visible in every project that shows this issue.")
		s.notes = append(s.notes, "Datas da organização são visíveis em todos os projetos que mostram a issue.")
	}
	for _, date := range []struct{ name, value string }{{capability.Start, start}, {capability.Target, target}} {
		if date.value == "" {
			continue
		}
		if err := s.writeDate(it, capability.Source, date.name, date.value); err != nil {
			return err
		}
	}
	return nil
}

func writeDateOrder(it domain.Item, capability *domain.DatesCapability, start, target string) error {
	for _, value := range []string{start, target} {
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			return domain.Errorf(domain.ExitUsage, "invalid date %q: use YYYY-MM-DD", value)
		}
	}
	if start == "" {
		start = domain.ItemDateText(it, capability, domain.DateStart)
	}
	if target == "" {
		target = domain.ItemDateText(it, capability, domain.DateTarget)
	}
	if start != "" && target != "" && start > target {
		return domain.Errorf(domain.ExitUsage, "start date must not follow target date")
	}
	return nil
}

func (s *writeSession) writeDate(it domain.Item, source domain.DateSource, name, value string) error {
	if name == "" {
		return writeUnavailable("date field")
	}
	if source == domain.DateSourceProject {
		field, err := domain.ResolveField(s.project, name)
		if err != nil {
			return err
		}
		if field.DataType != domain.DataTypeDate {
			return domain.Errorf(domain.ExitUsage, "date field %q must have type DATE", name)
		}
		return s.writeField(it, field, value)
	}
	id, err := s.writeIssueFieldID(it, name)
	if err != nil {
		return err
	}
	s.writeAdd(it, domain.OpSetIssueFieldValue, name, value, "Atualizar data da organização, visível em todos os projetos.",
		func(ctx context.Context, current domain.Item) error {
			input := domain.IssueFieldValueInput{IssueID: current.Issue.NodeID, FieldID: id, Value: value}
			if _, exists := current.IssueField(name); exists {
				return s.deps.Writer.UpdateIssueFieldValue(ctx, input)
			}
			return s.deps.Writer.SetIssueFieldValue(ctx, input)
		})
	return nil
}

func (s *writeSession) writeIssueFieldID(it domain.Item, name string) (string, error) {
	if field, ok := it.IssueField(name); ok && field.FieldID != "" {
		return field.FieldID, nil
	}
	if resolver, ok := s.deps.Reader.(writeIssueFieldResolver); ok {
		id, err := resolver.IssueFieldID(s.ctx, it.Issue.Owner, name)
		if err != nil {
			return "", writeAPI(err)
		}
		if id != "" {
			return id, nil
		}
	}
	return "", domain.Errorf(domain.ExitAPI, "organization issue field %q cannot be resolved; adapter must provide IssueFieldID", name)
}
