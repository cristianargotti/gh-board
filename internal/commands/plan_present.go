package commands

import (
	"strconv"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func planFormat(deps *Deps) (render.Format, error) {
	if deps.Flags.JSON {
		return render.FormatJSON, nil
	}
	if deps.Flags.Format == "" {
		return render.FormatTable, nil
	}
	return render.ParseFormat(deps.Flags.Format)
}

func planRender(deps *Deps, doc *render.Document) error {
	format, err := planFormat(deps)
	if err != nil {
		return err
	}
	return render.Render(deps.Out, doc, format)
}

func planMessage(deps *Deps, message string) error {
	doc := render.NewDocument("Plan")
	doc.AddSection("").AddNote(message)
	return planRender(deps, doc)
}

func planPresent(deps *Deps, p domain.Plan, path string) error {
	doc := render.NewDocument("Plan " + p.ID)
	safe := planDisplay(p)
	doc.Data = struct {
		Plan   domain.Plan `json:"plan"`
		Path   string      `json:"path,omitempty"`
		DryRun bool        `json:"dry_run"`
	}{safe, path, deps.Flags.DryRun}
	section := doc.AddSection("")
	section.AddKeyValue("Description", safe.Description).AddKeyValue("Actor", safe.Actor).AddKeyValue("Project", planProjectLine(safe))
	section.AddKeyValue("Expires", p.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
	if path != "" {
		section.AddKeyValue("File", path).AddNote("Run gh board apply " + p.ID + " in a terminal.")
	}
	table := doc.AddSection("Steps").SetTable("Step", "Operation", "Target", "Before", "After", "Description")
	for _, step := range safe.Steps {
		table.AddRow(strconv.Itoa(step.Index), string(step.Operation), step.Target.Title, step.Before, step.After, step.Description)
	}
	return planRender(deps, doc)
}

// planProjectLine names the project of a plan, or the project a copy step
// will create.
func planProjectLine(p domain.Plan) string {
	if !p.Project.IsZero() {
		return p.Project.String()
	}
	for _, step := range p.Steps {
		if step.Operation == domain.OpCopyProject {
			return "new project " + step.Target.Title
		}
	}
	return "new project"
}

// planDisplay prepares a plan for a reader: external text (titles,
// repositories, values) arrives delimited; the actor, the host and the
// command are the kit's own data and are only cleaned.
func planDisplay(p domain.Plan) domain.Plan {
	p.Description = render.Sanitize(p.Description, 0)
	p.Actor, p.Host, p.Command = render.Sanitize(p.Actor, 0), render.Sanitize(p.Host, 0), render.Sanitize(p.Command, render.ExcerptLimit)
	p.Steps = append([]domain.Step(nil), p.Steps...)
	for i := range p.Steps {
		step := &p.Steps[i]
		step.Target.Title = render.Title(step.Target.Title)
		step.Target.Repository = render.Title(step.Target.Repository)
		step.Field = render.Title(step.Field)
		step.Before, step.After = render.Excerpt(render.LabelBody, step.Before), render.Excerpt(render.LabelBody, step.After)
		step.Description = render.Sanitize(step.Description, 0)
	}
	return p
}
