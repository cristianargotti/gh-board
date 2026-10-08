package commands

import (
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

const (
	writeKindTask  = "task"
	writeKindEpic  = "epic"
	writeKindEntry = "entry"
)

func (s *writeSession) writeNewInput(kind string, repo domain.Repository, options *writeNewOptions) (domain.CreateIssueInput, domain.Item, error) {
	input := domain.CreateIssueInput{RepositoryID: repo.NodeID, Title: options.title}
	it := domain.Item{Issue: domain.Issue{Owner: repo.Owner, Repo: repo.Name, Title: options.title, State: domain.IssueOpen}}
	issueType, err := s.writeNewType(kind, repo.Owner)
	if err != nil {
		return input, it, err
	}
	if issueType != nil {
		it.Issue.Type, input.IssueTypeID = issueType, issueType.ID
	}
	if options.body != "" {
		input.Body, err = writeBody(s.cmd, options.body)
		if err != nil {
			return input, it, err
		}
	}
	if err := s.writeNewPeople(it, options, &input); err != nil {
		return input, it, err
	}
	if err := s.writeNewLabels(it, kind, options.labels, &input); err != nil {
		return input, it, err
	}
	if options.milestone != "" {
		milestones, err := s.deps.Reader.RepositoryMilestones(s.ctx, repo.Owner, repo.Name)
		if err != nil {
			return input, it, writeAPI(err)
		}
		milestone, err := domain.ResolveMilestone(milestones, options.milestone)
		if err != nil {
			return input, it, err
		}
		input.MilestoneID = milestone.ID
	}
	return input, it, nil
}

func (s *writeSession) writeNewType(kind, owner string) (*domain.IssueType, error) {
	name := ""
	switch kind {
	case writeKindTask, writeKindEntry:
		if s.cfg.Capabilities.Task != nil {
			name = s.cfg.Capabilities.Task.IssueType
		}
	case writeKindEpic:
		if s.cfg.Capabilities.Epic != nil {
			name = s.cfg.Capabilities.Epic.IssueType
		}
	default:
		return nil, domain.Errorf(domain.ExitUsage, "new requires task, epic, or entry")
	}
	if name == "" {
		// No issue type mapped (user-owned boards have none): a plain issue.
		return nil, nil
	}
	types, err := s.deps.Reader.IssueTypes(s.ctx, owner)
	if err != nil {
		return nil, writeAPI(err)
	}
	names := make([]string, 0, len(types))
	for _, it := range types {
		if strings.EqualFold(it.Name, name) {
			return &it, nil
		}
		names = append(names, it.Name)
	}
	return nil, domain.NotFound("issue type", name, names)
}

func (s *writeSession) writeNewPeople(it domain.Item, options *writeNewOptions, input *domain.CreateIssueInput) error {
	for _, login := range options.assignees {
		user, err := s.writeUser(it, login)
		if err != nil {
			return err
		}
		if user.ID == "" {
			return domain.Errorf(domain.ExitAPI, "resolved login has no node ID")
		}
		if !domain.Contains(input.AssigneeIDs, user.ID) {
			input.AssigneeIDs = append(input.AssigneeIDs, user.ID)
		}
	}
	return nil
}

func (s *writeSession) writeNewLabels(it domain.Item, kind string, names []string, input *domain.CreateIssueInput) error {
	names = append([]string(nil), names...)
	if kind == writeKindEntry {
		if s.cfg.Capabilities.Triage == nil || s.cfg.Capabilities.Triage.Label == "" {
			return writeUnavailable("triage")
		}
		names = append(names, s.cfg.Capabilities.Triage.Label)
	}
	if len(names) == 0 {
		return nil
	}
	labels, err := s.deps.Reader.RepositoryLabels(s.ctx, it.Issue.Owner, it.Issue.Repo)
	if err != nil {
		return writeAPI(err)
	}
	for _, name := range names {
		label, err := domain.ResolveLabel(labels, name)
		if err != nil {
			return err
		}
		if !domain.Contains(input.LabelIDs, label.ID) {
			input.LabelIDs = append(input.LabelIDs, label.ID)
		}
	}
	return nil
}
