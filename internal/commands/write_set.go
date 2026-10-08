package commands

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeSetCommand(deps *Deps) *cobra.Command {
	cmd := writeCommand("set <ref[,ref...]> field=value ...", "Set project fields; status uses move policy", cobra.MinimumNArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeSetFields(it, args[1:]) })
			})
		})
	cmd.Annotations = map[string]string{mcpArgsAnnotation: "ref: Issue references separated by commas: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number\n" +
		"fields...: Field assignments as field=value, one per entry, with the field and option names the board uses; the status field follows the move policy"}
	return cmd
}

func (s *writeSession) writeSetFields(it domain.Item, pairs []string) error {
	seen := map[string]bool{}
	for _, pair := range pairs {
		name, value, err := writePair(pair)
		if err != nil {
			return err
		}
		field, err := domain.ResolveField(s.project, name)
		if err != nil {
			return err
		}
		if seen[field.ID] {
			return domain.Errorf(domain.ExitUsage, "field %q specified more than once", field.Name)
		}
		seen[field.ID] = true
		if err := s.writeField(it, field, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *writeSession) writeField(it domain.Item, field domain.Field, value string) error {
	if err := s.writeCheckStatusConfig(); err != nil {
		return err
	}
	input, after, err := writeFieldInput(field, value)
	if err != nil {
		return err
	}
	if field.Name == domain.StatusFieldName(s.cfg) {
		if field.DataType != domain.DataTypeSingleSelect {
			return domain.Errorf(domain.ExitUsage, "status must be a single select field")
		}
		if err := s.writeMovePolicy(it, field, after); err != nil {
			return err
		}
		capability := s.cfg.Capabilities.Status
		if capability != nil && domain.Contains(capability.Done, after) {
			s.guarded = true
		}
	}
	s.writeAdd(it, domain.OpSetFieldValue, field.Name, after, "Atualizar campo do projeto.",
		func(ctx context.Context, current domain.Item) error {
			if current.ProjectItemID == "" {
				return domain.Errorf(domain.ExitNotFound, "target is not in the project")
			}
			input.ProjectID, input.ItemID = s.project.NodeID, current.ProjectItemID
			return s.deps.Writer.UpdateItemFieldValue(ctx, input)
		})
	return nil
}

func writeFieldInput(field domain.Field, value string) (domain.ItemFieldValueInput, string, error) {
	input := domain.ItemFieldValueInput{FieldID: field.ID}
	if strings.TrimSpace(value) == "" {
		return input, value, domain.Errorf(domain.ExitUsage, "clearing fields is unavailable")
	}
	switch field.DataType {
	case domain.DataTypeText:
		input.Text = &value
	case domain.DataTypeNumber:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return input, value, domain.Errorf(domain.ExitUsage, "invalid number %q", value)
		}
		input.Number = &number
		value = strconv.FormatFloat(number, 'f', -1, 64)
	case domain.DataTypeDate:
		date, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return input, value, domain.Errorf(domain.ExitUsage, "invalid date %q: use YYYY-MM-DD", value)
		}
		input.Date = &date
	case domain.DataTypeSingleSelect:
		option, err := domain.ResolveOption(field, value)
		if err != nil {
			return input, value, err
		}
		input.SingleSelectID, value = &option.ID, option.Name
	case domain.DataTypeIteration:
		iteration, err := domain.ResolveIteration(field, value)
		if err != nil {
			return input, value, err
		}
		input.IterationID, value = &iteration.ID, iteration.Title
	default:
		return input, value, domain.Errorf(domain.ExitUsage, "field %q is not directly writable; use its dedicated command", field.Name)
	}
	return input, value, nil
}

func (s *writeSession) writeMappedField(it domain.Item, capability *domain.FieldCapability, role, value string) error {
	if capability == nil || capability.Field == "" {
		return writeUnavailable(role)
	}
	field, err := domain.ResolveField(s.project, capability.Field)
	if err != nil {
		return err
	}
	return s.writeField(it, field, value)
}
