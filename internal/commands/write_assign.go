package commands

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/spf13/cobra"
)

type (
	writeUserResolver interface {
		UserID(context.Context, string) (string, error)
	}
	writeAssignableUsers interface {
		AssignableUsers(context.Context, string, string) ([]domain.User, error)
	}
)

func writeAssignCommand(deps *Deps) *cobra.Command {
	cmd := writeCommand("assign <ref[,ref...]> <login>", "Add an assignee", cobra.ExactArgs(2),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error {
				return s.writeTargets(args[0], func(it domain.Item) error { return s.writeAssignee(it, args[1], true) })
			})
		})
	cmd.Annotations = map[string]string{mcpArgsAnnotation: "ref: Issue references separated by commas: owner/repo#n, a GitHub URL or a node id; #n alone works when board.yml names a repository and one item carries that number\n" +
		"login: GitHub login of the person to add, with or without the leading @"}
	return cmd
}

func (s *writeSession) writeUser(it domain.Item, login string) (domain.User, error) {
	login = strings.TrimPrefix(login, "@")
	users := append([]domain.User{{ID: s.viewer.ID, Login: s.viewer.Login}}, it.Issue.Assignees...)
	if resolver, ok := s.deps.Reader.(writeAssignableUsers); ok {
		known, err := resolver.AssignableUsers(s.ctx, it.Issue.Owner, it.Issue.Repo)
		if err != nil {
			return domain.User{}, writeAPI(err)
		}
		users = append(users, known...)
	}
	if user, err := domain.ResolveUser(users, login); err == nil {
		return user, nil
	}
	if resolver, ok := s.deps.Reader.(writeUserResolver); ok {
		id, err := resolver.UserID(s.ctx, login)
		if err == nil && id != "" {
			return domain.User{ID: id, Login: login}, nil
		}
		if err != nil && domain.CodeOf(err) != domain.ExitNotFound {
			return domain.User{}, writeAPI(err)
		}
	}
	return domain.ResolveUser(users, login)
}

func (s *writeSession) writeAssignee(it domain.Item, login string, add bool) error {
	user, err := s.writeUser(it, login)
	if err != nil {
		return err
	}
	if user.ID == "" {
		return domain.Errorf(domain.ExitAPI, "resolved login has no node ID")
	}
	op := domain.OpRemoveAssignees
	if add {
		op = domain.OpAddAssignees
	}
	after := writeChangeSet(domain.EncodeAssignees(it.Issue.Assignees), []string{user.Login}, add)
	s.writeAdd(it, op, "assignees", after, "Atualizar responsáveis.", func(ctx context.Context, current domain.Item) error {
		if add {
			return s.deps.Writer.AddAssignees(ctx, current.Issue.NodeID, []string{user.ID})
		}
		return s.deps.Writer.RemoveAssignees(ctx, current.Issue.NodeID, []string{user.ID})
	})
	return nil
}

func writeChangeSet(before string, names []string, add bool) string {
	values := plan.SplitSet(before)
	for _, name := range names {
		filtered := values[:0]
		for _, existing := range values {
			if !strings.EqualFold(existing, name) {
				filtered = append(filtered, existing)
			}
		}
		values = filtered
		if add {
			values = append(values, name)
		}
	}
	return plan.JoinSet(values)
}
