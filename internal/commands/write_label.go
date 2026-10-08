package commands

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeLabelCommand(deps *Deps) *cobra.Command {
	cmd := writeCommand("label <ref[,ref...]> +name -name ...", "Add or remove repository labels", nil,
		func(cmd *cobra.Command, args []string) error {
			args, err := writeLabelArgs(cmd, args)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeLabels(it, args[1:]) })
			})
		})
	cmd.DisableFlagParsing = true
	return cmd
}

func writeLabelArgs(cmd *cobra.Command, args []string) ([]string, error) {
	var flags, labels []string
	positional := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = true
			continue
		}
		if !positional && strings.HasPrefix(arg, "--") {
			flags = append(flags, arg)
			name, _, inline := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			flag := cmd.InheritedFlags().Lookup(name)
			if flag != nil && flag.NoOptDefVal == "" && !inline && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			labels = append(labels, arg)
		}
	}
	cmd.DisableFlagParsing = false
	defer func() { cmd.DisableFlagParsing = true }()
	if err := cmd.ParseFlags(flags); err != nil {
		return nil, err
	}
	if help, _ := cmd.Flags().GetBool("help"); help {
		return nil, nil
	}
	if err := cobra.MinimumNArgs(2)(cmd, labels); err != nil {
		return nil, err
	}
	return labels, nil
}

func (s *writeSession) writeLabels(it domain.Item, changes []string) error {
	labels, err := s.deps.Reader.RepositoryLabels(s.ctx, it.Issue.Owner, it.Issue.Repo)
	if err != nil {
		return writeAPI(err)
	}
	add, remove, err := writeLabelChanges(labels, changes)
	if err != nil {
		return err
	}
	after := domain.EncodeLabels(it.Issue.Labels)
	for _, group := range []struct {
		labels []domain.Label
		add    bool
	}{{add, true}, {remove, false}} {
		if len(group.labels) == 0 {
			continue
		}
		after = s.writeLabelAction(it, group.labels, group.add, after)
	}
	return nil
}

func writeLabelChanges(labels []domain.Label, changes []string) ([]domain.Label, []domain.Label, error) {
	var add, remove []domain.Label
	seen := map[string]bool{}
	for _, change := range changes {
		if len(change) < 2 || (change[0] != '+' && change[0] != '-') {
			return nil, nil, domain.Errorf(domain.ExitUsage, "labels need +name or -name")
		}
		label, err := domain.ResolveLabel(labels, change[1:])
		if err != nil {
			return nil, nil, err
		}
		if seen[label.ID] {
			return nil, nil, domain.Errorf(domain.ExitUsage, "label %q specified more than once", label.Name)
		}
		seen[label.ID] = true
		if change[0] == '+' {
			add = append(add, label)
		} else {
			remove = append(remove, label)
		}
	}
	return add, remove, nil
}

func (s *writeSession) writeLabelAction(it domain.Item, labels []domain.Label, add bool, before string) string {
	var ids, names []string
	for _, label := range labels {
		ids = append(ids, label.ID)
		names = append(names, label.Name)
	}
	op := domain.OpRemoveLabels
	if add {
		op = domain.OpAddLabels
	}
	after := writeChangeSet(before, names, add)
	s.writeAdd(it, op, "labels", after, "Atualizar etiquetas.", func(ctx context.Context, current domain.Item) error {
		if add {
			return s.deps.Writer.AddLabels(ctx, current.Issue.NodeID, ids)
		}
		return s.deps.Writer.RemoveLabels(ctx, current.Issue.NodeID, ids)
	})
	return after
}
