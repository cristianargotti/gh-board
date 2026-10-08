package domain

import "time"

func (p *tidyPlanner) sprint(it Item) {
	const rule = "sprint_from_target"
	if p.caps.Sprint == nil || p.caps.Dates == nil {
		p.skip(it, rule, "sprint ou datas não mapeadas em board.yml")
		return
	}
	target, ok := ItemDate(it, p.caps.Dates, DateTarget, p.loc)
	if !ok {
		p.skip(it, rule, "data alvo ausente ou inválida")
		return
	}
	field, ok := p.in.Project.FieldByName(p.caps.Sprint.Field)
	if !ok || field.DataType != DataTypeIteration {
		p.skip(it, rule, "campo de sprint não encontrado como iteração")
		return
	}
	matches := tidyIterationsAt(field.Iterations, target)
	if len(matches) != 1 {
		p.skip(it, rule, "data alvo sem uma única iteração correspondente")
		return
	}
	p.projectStep(it, field.Name, matches[0].Title, rule, "Definir sprint pela data alvo")
}

// tidyIterationsAt lists the iterations whose calendar span holds the
// target date.
func tidyIterationsAt(iterations []Iteration, target time.Time) []Iteration {
	result := []Iteration{}
	for _, iteration := range iterations {
		if !iteration.Start.IsZero() && iteration.Contains(target) {
			result = append(result, iteration)
		}
	}
	return result
}

func (p *tidyPlanner) start(it Item) {
	const rule = "stamp_start_on_active"
	if !IsActive(p.in.Config, it) {
		return
	}
	if p.caps.Dates == nil || p.caps.Dates.Start == "" {
		p.skip(it, rule, "data de início não mapeada em board.yml")
		return
	}
	if ItemDateText(it, p.caps.Dates, DateStart) != "" {
		return
	}
	status, _ := it.Value(p.caps.Status.Field)
	if status.UpdatedAt.IsZero() || p.in.Now.IsZero() || status.UpdatedAt.After(p.in.Now) {
		p.skip(it, rule, "entrada no status ativo sem data válida")
		return
	}
	date := DateText(Today(status.UpdatedAt, p.loc))
	field := p.caps.Dates.Start
	if p.caps.Dates.Source == DateSourceIssueFields {
		p.step(it, OpSetIssueFieldValue, field, "", date,
			"Registrar início pela entrada no status ativo; campo da issue visível em todos os projetos")
		return
	}
	p.projectStep(it, field, date, rule, "Registrar início pela entrada no status ativo")
}
