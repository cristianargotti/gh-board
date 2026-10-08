package domain

import "strings"

const tidyEpicRule = "epic_follows_tasks"

func (p *tidyPlanner) epic(it Item) {
	if p.caps.Status == nil || p.caps.Task == nil || p.caps.Task.IssueType == "" {
		p.skip(it, tidyEpicRule, "status ou tarefa não mapeados em board.yml")
		return
	}
	tasks, complete := p.tasks(it)
	if !complete || len(tasks) == 0 {
		p.skip(it, tidyEpicRule, "tarefas ausentes ou conjunto de sub-issues incompleto")
		return
	}
	class := tidyTaskClass(p.in.Config, tasks)
	if p.caps.Status.Classify(it.Text(p.caps.Status.Field)) == class && class != StatusUnknown {
		return
	}
	options := tidyClassOptions(p.caps.Status, class)
	if len(options) != 1 {
		p.skip(it, tidyEpicRule, "tarefas sem status conhecido ou classe sem opção única")
		return
	}
	before, after := it.Text(p.caps.Status.Field), options[0]
	if err := CheckMove(p.in.Config.Policy, before, after, p.counts[after]); err != nil {
		p.skip(it, tidyEpicRule, "mudança recusada pela política de transição ou WIP")
		return
	}
	why := "Atualizar status do épico conforme suas tarefas"
	if class == StatusDone {
		why += "; pode fechar a issue pelo workflow do projeto"
	}
	if p.projectStep(it, p.caps.Status.Field, after, tidyEpicRule, why) {
		p.counts[before]--
		p.counts[after]++
	}
}

func (p *tidyPlanner) tasks(parent Item) ([]Item, bool) {
	tasks := []Item{}
	observed := 0
	for _, it := range p.in.Items {
		if !tidyChildOf(it.Issue.Parent, parent) {
			continue
		}
		observed++
		if it.Issue.Type == nil || !strings.EqualFold(it.Issue.Type.Name, p.caps.Task.IssueType) {
			continue
		}
		if !IsEpic(p.caps, it) {
			tasks = append(tasks, it)
		}
	}
	return tasks, observed >= parent.Issue.SubIssues.Total
}

func tidyTaskClass(cfg *Config, tasks []Item) StatusClass {
	counts := map[StatusClass]int{}
	for _, task := range tasks {
		_, class := ItemStatus(cfg, task)
		if task.Issue.State == IssueClosed {
			class = StatusDone
		}
		counts[class]++
	}
	switch {
	case counts[StatusUnknown] > 0 || len(tasks) == 0:
		return StatusUnknown
	case counts[StatusDone] == len(tasks):
		return StatusDone
	case counts[StatusActive] > 0 || counts[StatusDone] > 0:
		return StatusActive
	case counts[StatusReady] > 0:
		return StatusReady
	default:
		return StatusBacklog
	}
}

func tidyClassOptions(status *StatusCapability, class StatusClass) []string {
	switch class {
	case StatusBacklog:
		return status.Backlog
	case StatusReady:
		return status.Ready
	case StatusActive:
		return status.Active
	case StatusDone:
		return status.Done
	default:
		return nil
	}
}
