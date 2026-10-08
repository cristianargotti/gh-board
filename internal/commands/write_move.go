package commands

import (
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

func writeMoveCommand(deps *Deps) *cobra.Command {
	cmd := writeCommand("move <ref[,ref...]> <status>", "Move work, saving a plan for a done status", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				field, ok := domain.StatusField(s.cfg, s.project)
				if !ok {
					return writeUnavailable("status")
				}
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeField(it, field, args[1]) })
			})
		})
	cmd.Annotations = map[string]string{mcpArgsAnnotation: "ref: Issue references separated by commas: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number\n" +
		"status: Target status option as the board names it; a done status turns the call into a plan"}
	return cmd
}

func (s *writeSession) writeMovePolicy(it domain.Item, field domain.Field, after string) error {
	from := it.Text(field.Name)
	if err := domain.CheckTransition(s.cfg.Policy, from, after); err != nil {
		allowed, _ := s.cfg.Policy.AllowedTransitions(from)
		return domain.Errorf(domain.ExitPolicy, "transition from %q to %q is not allowed by board.yml (allowed: %s)", from, after, writeAllowedList(allowed))
	}
	occupied := 0
	if _, limited := s.cfg.Policy.WIPLimit(after); limited {
		items, err := s.writeAllItems()
		if err != nil {
			return err
		}
		occupied = s.writeOccupied(items, it.Issue.NodeID, field.Name, after)
	}
	if err := domain.CheckWIP(s.cfg.Policy, after, occupied); err != nil {
		limit, _ := s.cfg.Policy.WIPLimit(after)
		return domain.Errorf(domain.ExitPolicy, "WIP limit for %q reached: %d of %d", after, occupied, limit)
	}
	return nil
}

// writeAllowedList names the destinations a transition rule allows.
func writeAllowedList(allowed []string) string {
	if len(allowed) == 0 {
		return "none"
	}
	return strings.Join(allowed, ", ")
}

func (s *writeSession) writeOccupied(items []domain.Item, exclude, field, status string) int {
	count := 0
	for _, it := range items {
		if it.Issue.NodeID == exclude || it.Archived || !it.IsOpen() {
			continue
		}
		value := it.Text(field)
		for _, action := range s.actions {
			if action.step.Target.NodeID == it.Issue.NodeID && action.step.Field == field {
				value = action.step.After
			}
		}
		if value == status {
			count++
		}
	}
	return count
}

func (s *writeSession) writeAllItems() ([]domain.Item, error) {
	var items []domain.Item
	cursor := ""
	seen := map[string]bool{}
	for {
		page, err := s.deps.Reader.ListItems(s.ctx, s.project, domain.ListOptions{All: true, Cursor: cursor})
		if err != nil {
			return nil, writeAPI(err)
		}
		items = append(items, page.Items...)
		if !page.HasNext {
			return items, nil
		}
		if page.NextCursor == "" || seen[page.NextCursor] {
			return nil, domain.Errorf(domain.ExitAPI, "incomplete item pagination")
		}
		cursor = page.NextCursor
		seen[cursor] = true
	}
}

func (s *writeSession) writeRecheckMove(step domain.Step) error {
	if step.Operation != domain.OpSetFieldValue || step.Field != domain.StatusFieldName(s.cfg) {
		return nil
	}
	field, _ := domain.StatusField(s.cfg, s.project)
	it := s.items[step.Target.NodeID]
	return s.writeMovePolicy(it, field, step.After)
}
