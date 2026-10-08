package commands

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/spf13/cobra"
)

type writeNewOptions struct {
	title, body, parent, lane, epic, sprint, estimate, start, target, milestone string
	assignees, labels                                                           []string
}

type writeRepositoryResolver interface {
	RepositoryID(context.Context, string, string) (string, error)
}

func writeNewCommand(deps *Deps) *cobra.Command {
	options := &writeNewOptions{}
	cmd := writeCommand("new <task|epic|entry>", "Create an issue and add it to the project", cobra.ExactArgs(1),
		func(cmd *cobra.Command, args []string) error {
			return writeRun(cmd, deps, func(s *writeSession) error { return s.writeNewIssue(args[0], options) })
		})
	cmd.ValidArgs = []string{writeKindTask, writeKindEpic, writeKindEntry}
	cmd.Annotations = map[string]string{mcpArgsAnnotation: "kind: What to create: task (the configured task type), epic (the configured epic type) or entry (a task carrying the triage label)"}
	flags := cmd.Flags()
	flags.StringVar(&options.title, "title", "", "issue title")
	flags.StringVar(&options.body, "body", "", "body text, file path, @file, or - for standard input")
	flags.StringVar(&options.parent, "parent", "", "parent issue reference")
	flags.StringVar(&options.lane, "lane", "", "configured lane value")
	flags.StringVar(&options.epic, "epic", "", "configured epic field value")
	flags.StringVar(&options.sprint, "sprint", "", "current, next, or iteration title")
	flags.StringVar(&options.estimate, "estimate", "", "estimate in days")
	flags.StringVar(&options.start, "start", "", "start date, YYYY-MM-DD")
	flags.StringVar(&options.target, "target", "", "target date, YYYY-MM-DD")
	flags.StringVar(&options.milestone, "milestone", "", "repository milestone title")
	flags.StringSliceVar(&options.assignees, "assignee", nil, "assignee login, repeatable or comma separated")
	flags.StringSliceVar(&options.labels, "label", nil, "label name, repeatable or comma separated")
	return cmd
}

func (s *writeSession) writeNewIssue(kind string, options *writeNewOptions) error {
	if err := s.writeNewValidate(options); err != nil {
		return err
	}
	repo, err := s.writeRepository()
	if err != nil {
		return err
	}
	input, it, err := s.writeNewInput(kind, repo, options)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	s.writeAdd(it, domain.OpCreateIssue, "", string(encoded), "Criar issue no repositório configurado.",
		func(ctx context.Context, _ domain.Item) error { return s.writeCreate(ctx, input) })
	s.writeAdd(it, domain.OpAddProjectItem, "", s.project.NodeID, "Adicionar issue ao projeto.",
		func(ctx context.Context, current domain.Item) error {
			id, err := s.deps.Writer.AddProjectItem(ctx, s.project.NodeID, current.Issue.NodeID)
			if err != nil {
				return err
			}
			if id == "" {
				return domain.Errorf(domain.ExitAPI, "project item creation returned no node ID")
			}
			s.writeBindCreated(current.Issue, id)
			return nil
		})
	if err := s.writeNewFields(it, options); err != nil {
		return err
	}
	noun := map[string]string{writeKindTask: "tarefa", writeKindEpic: "épico", writeKindEntry: "entrada"}[kind]
	s.summary = "Criar " + noun + " e configurar seu trabalho no quadro."
	return nil
}

func (s *writeSession) writeRepository() (domain.Repository, error) {
	owner, name, ok := strings.Cut(s.cfg.Repository, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return domain.Repository{}, domain.Errorf(domain.ExitUsage, "new requires repository: owner/name in board.yml")
	}
	if err := s.writePermission(owner, name); err != nil {
		return domain.Repository{}, err
	}
	for _, repo := range s.project.Repositories {
		if repo.FullName() == s.cfg.Repository && repo.NodeID != "" {
			return repo, nil
		}
	}
	if resolver, ok := s.deps.Reader.(writeRepositoryResolver); ok {
		id, err := resolver.RepositoryID(s.ctx, owner, name)
		if err != nil {
			return domain.Repository{}, writeAPI(err)
		}
		if id != "" {
			return domain.Repository{Owner: owner, Name: name, NodeID: id}, nil
		}
	}
	return domain.Repository{}, domain.Errorf(domain.ExitNotFound, "configured repository is not discovered; adapter must provide RepositoryID")
}

func (s *writeSession) writeCreate(ctx context.Context, input domain.CreateIssueInput) error {
	issue, err := s.deps.Writer.CreateIssue(ctx, input)
	if err != nil {
		return err
	}
	if issue.NodeID == "" {
		return domain.Errorf(domain.ExitAPI, "issue creation returned no node ID")
	}
	s.writeBindCreated(issue, "")
	return nil
}

func (s *writeSession) writeBindCreated(issue domain.Issue, itemID string) {
	for i := range s.actions {
		target := &s.actions[i].step.Target
		target.NodeID, target.ProjectItemID = issue.NodeID, itemID
		target.Number, target.Title = issue.Number, issue.Title
		if i < len(s.plan.Steps) {
			s.plan.Steps[i].Target = *target
		}
	}
}

func (s *writeSession) writeNewValidate(options *writeNewOptions) error {
	if strings.TrimSpace(options.title) == "" {
		return domain.Errorf(domain.ExitUsage, "provide a nonempty --title")
	}
	if len(s.deps.Flags.Expect) > 0 {
		return domain.Errorf(domain.ExitUsage, "new has no existing target for --expect")
	}
	if _, ok := s.deps.Reader.(writeIssueReader); !ok {
		return domain.Errorf(domain.ExitAPI, "new requires adapter IssueByID to reread the issue before adding it to the project")
	}
	return nil
}
