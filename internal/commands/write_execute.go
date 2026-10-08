package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
	"github.com/cristianargotti/gh-board/internal/render"
)

type writeFuncItem = func(context.Context, domain.Item) error

func (s *writeSession) writeExecute() error {
	if len(s.actions) == 0 {
		return s.writeNothing()
	}
	if err := s.writePreflight(); err != nil {
		return err
	}
	if err := s.writePreparePlan(); err != nil {
		return err
	}
	if s.guarded {
		return s.writeRequired()
	}
	if s.deps.Flags.DryRun {
		return s.writeOutput("Dry run")
	}
	for i := range s.actions {
		if err := s.writePerform(i); err != nil {
			return err
		}
	}
	return s.writeOutput("Write completed")
}

func (s *writeSession) writePreflight() error {
	seen := map[string]domain.Item{}
	for _, action := range s.actions {
		id := action.step.Target.NodeID
		if id == "" {
			continue
		}
		it, ok := seen[id]
		if !ok {
			var err error
			it, err = s.writeFresh(action.step.Target)
			if err != nil {
				return err
			}
			seen[id] = it
		}
		if err := s.writeCompare(action.step, it); err != nil {
			return err
		}
	}
	return nil
}

func (s *writeSession) writeFresh(target domain.Target) (domain.Item, error) {
	it, err := s.writeReadTarget(target)
	if err != nil {
		return it, writeAPI(err)
	}
	if it.Issue.NodeID != target.NodeID || (target.ProjectItemID != "" && it.ProjectItemID != target.ProjectItemID) {
		return it, domain.Errorf(domain.ExitDrift, "resolved target changed before write")
	}
	return it, s.writeCheckExpect(it)
}

func (s *writeSession) writeCompare(step domain.Step, it domain.Item) error {
	if value, ok := plan.Precondition(step, it); ok && value != step.Before {
		return domain.Errorf(domain.ExitDrift, "%s %s: expected %q, found %q", it.Issue.Ref(), step.Field, step.Before, value)
	}
	return nil
}

func (s *writeSession) writePerform(index int) error {
	action := &s.actions[index]
	it := domain.Item{}
	err := s.writeRecheckMove(action.step)
	if err == nil && action.step.Target.NodeID != "" {
		it, err = s.writeFresh(action.step.Target)
		if err == nil {
			err = s.writeCompare(action.step, it)
		}
	}
	if err == nil {
		err = writeAPI(action.run(s.ctx, it))
	}
	journalErr := s.writeJournal(*action, err)
	if err == nil {
		s.writeAdvance(action.step)
	}
	return errors.Join(err, journalErr)
}

func (s *writeSession) writeAdvance(step domain.Step) {
	for field := range s.expect[step.Target.NodeID] {
		condition, err := writeCondition(s.cfg, s.project, s.items[step.Target.NodeID], field, "")
		if err == nil && strings.EqualFold(condition.Field, step.Field) && writeConditionOperation(condition.Operation) == writeConditionOperation(step.Operation) {
			s.expect[step.Target.NodeID][field] = step.After
		}
	}
	for i := step.Index + 1; i < len(s.actions); i++ {
		next := &s.actions[i].step
		if next.Target.NodeID == step.Target.NodeID && next.Field == step.Field {
			next.Before = step.After
		}
	}
}

// writeNothing reports a write whose every target is already in the
// requested state: no plan record, no journal, no mutation.
func (s *writeSession) writeNothing() error {
	doc := render.NewDocument("Nothing to do")
	doc.Data = struct {
		Plan     domain.Plan `json:"plan"`
		DryRun   bool        `json:"dry_run"`
		Required bool        `json:"plan_required"`
		Nothing  bool        `json:"nothing_to_do"`
	}{s.plan, s.deps.Flags.DryRun, false, true}
	section := doc.AddSection("")
	for id := range s.items {
		section.AddItem(s.items[id].Issue.Ref() + " is already in the requested state")
	}
	format, _ := render.ParseFormat(s.deps.Flags.Format)
	if s.deps.Flags.JSON {
		format = render.FormatJSON
	}
	return render.Render(s.cmd.OutOrStdout(), doc, format)
}

func (s *writeSession) writeRequired() error {
	if err := s.writeOutput("Plan required"); err != nil {
		return err
	}
	if s.deps.Flags.DryRun {
		return domain.Errorf(domain.ExitPlanRequired, "dry run: a plan is required; no plan was saved")
	}
	return domain.Errorf(domain.ExitPlanRequired, "human approval required; run exactly: gh board apply %s", s.plan.ID)
}

func (s *writeSession) writeOutput(title string) error {
	doc := render.NewDocument(title)
	doc.Data = struct {
		Plan     domain.Plan `json:"plan"`
		DryRun   bool        `json:"dry_run"`
		Required bool        `json:"plan_required"`
	}{writeDisplayPlan(s.plan), s.deps.Flags.DryRun, s.guarded}
	section := doc.AddSection("")
	section.AddKeyValue("Plan", s.plan.ID)
	if s.guarded && !s.deps.Flags.DryRun {
		section.AddNote("Run: gh board apply " + s.plan.ID)
	}
	for _, step := range s.plan.Steps {
		section.AddItem(fmt.Sprintf("%s %s %s: %s -> %s", step.Operation, render.Title(step.Target.Title),
			render.Sanitize(step.Field, 120), render.Excerpt("before", step.Before), render.Excerpt("after", step.After)))
	}
	format, _ := render.ParseFormat(s.deps.Flags.Format)
	if s.deps.Flags.JSON {
		format = render.FormatJSON
	}
	return render.Render(s.cmd.OutOrStdout(), doc, format)
}

func (s *writeSession) writeReadTarget(target domain.Target) (domain.Item, error) {
	if target.ProjectItemID == "" && len(s.actions) > 0 && s.actions[0].step.Operation == domain.OpCreateIssue {
		if reader, ok := s.deps.Reader.(writeIssueReader); ok {
			issue, err := reader.IssueByID(s.ctx, target.NodeID)
			return domain.Item{Issue: issue}, err
		}
	}
	return s.deps.Reader.GetItem(s.ctx, s.project, domain.Reference{Kind: domain.RefNodeID, NodeID: target.NodeID})
}

func (s *writeSession) writePreparePlan() error {
	s.plan.Description = s.writeDescription()
	for _, action := range s.actions {
		step := action.step
		if step.Index > 0 && step.Target.NodeID == "" {
			step.Target.NodeID = plan.Ref(0)
			if step.Operation != domain.OpAddProjectItem {
				step.Target.ProjectItemID = plan.Ref(1)
			}
		}
		s.plan.Steps = append(s.plan.Steps, step)
	}
	if err := s.writePlanConditions(); err != nil {
		return err
	}
	if !s.deps.Flags.DryRun {
		if _, err := plan.Write(s.deps.Dirs.State, s.plan); err != nil {
			return err
		}
	}
	return nil
}

func (s *writeSession) writeJournal(action writeAction, err error) error {
	status := plan.StatusDone
	if err != nil {
		status = plan.StatusFailed
	}
	target := action.step.Target
	entry := plan.JournalEntry{
		PlanID: s.plan.ID, Step: action.step.Index, Operation: action.step.Operation,
		Target: target, Status: status, At: s.deps.Now(),
	}
	if err != nil {
		entry.Error = err.Error()
	} else {
		entry.Created = writeCreatedNode(action.step)
	}
	return plan.NewJournal(s.deps.Dirs.State).Append(entry)
}

func writeDisplayPlan(p domain.Plan) domain.Plan {
	p.Steps = append([]domain.Step(nil), p.Steps...)
	for i := range p.Steps {
		p.Steps[i].Target.Title = render.Title(p.Steps[i].Target.Title)
		p.Steps[i].Before = render.Excerpt("before", p.Steps[i].Before)
		p.Steps[i].After = render.Excerpt("after", p.Steps[i].After)
	}
	return p
}

func writeCreatedNode(step domain.Step) string {
	switch step.Operation {
	case domain.OpCreateIssue:
		return step.Target.NodeID
	case domain.OpAddProjectItem:
		return step.Target.ProjectItemID
	default:
		return ""
	}
}
