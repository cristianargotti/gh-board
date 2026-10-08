package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readSchemaRefresher is the narrow port schema --refresh needs: a
// discovery that bypasses the schema cache. The adapter gains it at
// integration; until then --refresh reads through the normal discovery
// and says so.
type readSchemaRefresher interface {
	RefreshProject(ctx context.Context, ref domain.ProjectRef) (domain.Project, error)
}

// readSchemaPayload is the --json form of schema.
type readSchemaPayload struct {
	GeneratedAt string         `json:"generated_at"`
	Refreshed   bool           `json:"refreshed"`
	Note        string         `json:"note,omitempty"`
	Project     domain.Project `json:"project"`
}

func readSchemaCommand(deps *Deps) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:     "schema",
		Short:   "Discovered board schema: fields, options, iterations, views, workflows, repositories",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readSchema(cmd.Context(), deps, refresh) },
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "discard the cached schema and discover again")
	return cmd
}

func readSchema(ctx context.Context, deps *Deps, refresh bool) error {
	if deps.Reader == nil {
		return domain.Errorf(domain.ExitUsage, "a GitHub reader is required")
	}
	loaded, cfg, err := readLoad(deps)
	if err != nil {
		return err
	}
	if cfg.Project.IsZero() {
		return readNoProject(loaded)
	}
	project, refreshed, note, err := readDiscover(ctx, deps, cfg.Project, refresh)
	if err != nil {
		return err
	}
	payload := readSchemaPayload{
		GeneratedAt: readStamp(deps.Now()), Refreshed: refreshed, Note: note, Project: readSchemaDisplay(project),
	}
	doc := readSchemaDocument(payload, domain.Today(deps.Now(), domain.LocationOf(cfg)))
	doc.Data = payload
	return readRender(deps, doc)
}

// readDiscover reads the schema, through the refresher when asked and
// available.
func readDiscover(ctx context.Context, deps *Deps, ref domain.ProjectRef, refresh bool) (domain.Project, bool, string, error) {
	note := ""
	if refresh {
		if r, ok := deps.Reader.(readSchemaRefresher); ok {
			p, err := r.RefreshProject(ctx, ref)
			return p, true, "", readAPI(err)
		}
		note = "refresh not supported by the adapter: schema read through the normal discovery"
	}
	p, err := deps.Reader.DiscoverProject(ctx, ref)
	return p, false, note, readAPI(err)
}

// readSchemaDisplay delimits the external texts of the project: the title
// and the README excerpt.
func readSchemaDisplay(p domain.Project) domain.Project {
	p.Title = render.Title(p.Title)
	p.README = render.Excerpt(render.LabelReadme, p.README)
	p.DiscoveredAt = p.DiscoveredAt.UTC().Truncate(time.Second)
	return p
}

// readSchemaDocument lays the schema out; day is the board-zone date of
// the run, which places every iteration as completed, current or upcoming.
func readSchemaDocument(p readSchemaPayload, day time.Time) *render.Document {
	project := p.Project
	doc := render.NewDocument("Schema " + project.Ref.String())
	sec := doc.AddSection(readKeyProject)
	sec.AddKeyValue(readKeyTitle, project.Title).AddKeyValue("Node id", project.NodeID)
	sec.AddKeyValue(readKeyItems, fmt.Sprint(project.ItemCount)).AddKeyValue(readKeyRole, string(project.ViewerRole))
	sec.AddKeyValue("Discovered at", readStamp(project.DiscoveredAt)).AddKeyValue("Refreshed", readYesNo(p.Refreshed))
	repos := make([]string, 0, len(project.Repositories))
	for _, r := range project.Repositories {
		repos = append(repos, r.FullName())
	}
	sec.AddKeyValue("Repositories", strings.Join(repos, ", ")).AddKeyValue("README", project.README)
	if p.Note != "" {
		sec.AddNote(p.Note)
	}
	readSchemaFields(doc, project, day)
	views := doc.AddSection("Views").SetTable(readColName, "LAYOUT", "FILTER")
	for _, v := range project.Views {
		views.AddRow(v.Name, v.Layout, v.Filter)
	}
	workflows := doc.AddSection("Workflows").SetTable(readColName, "ENABLED")
	for _, w := range project.Workflows {
		workflows.AddRow(w.Name, readYesNo(w.Enabled))
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, p.GeneratedAt)
	return doc
}

func readSchemaFields(doc *render.Document, project domain.Project, day time.Time) {
	fields := doc.AddSection("Fields").SetTable(readColName, "TYPE", "OPTIONS")
	iterations := doc.AddSection("Iterations").SetTable("FIELD", readColTitle, readColStart, "END", readColState)
	for _, f := range project.Fields {
		fields.AddRow(f.Name, string(f.DataType), strings.Join(f.OptionNames(), ", "))
		for _, it := range f.Iterations {
			iterations.AddRow(f.Name, it.Title, domain.DateText(it.Start), domain.DateText(it.LastDay()), readIterationState(it, day))
		}
	}
}

// readIterationState says where an iteration stands on a calendar date.
func readIterationState(it domain.Iteration, day time.Time) string {
	switch {
	case it.Completed || !day.Before(it.End()):
		return "completed"
	case it.Contains(day):
		return readSprintCurrent
	default:
		return "upcoming"
	}
}
