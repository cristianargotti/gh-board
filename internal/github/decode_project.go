package github

import (
	"sort"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Owner kinds of a project.
const (
	ownerOrganization = "Organization"
	ownerUser         = "User"
)

// rawNodes is a connection read without pagination.
type rawNodes[T any] struct {
	Nodes []T `json:"nodes"`
}

// rawProject is the ProjectSchema fragment.
type rawProject struct {
	ID              string `json:"id"`
	Number          int    `json:"number"`
	Title           string `json:"title"`
	Readme          string `json:"readme"`
	URL             string `json:"url"`
	ViewerCanUpdate bool   `json:"viewerCanUpdate"`
	Owner           struct {
		TypeName            string `json:"__typename"`
		Login               string `json:"login"`
		ViewerCanAdminister bool   `json:"viewerCanAdminister"`
	} `json:"owner"`
	Items struct {
		TotalCount int `json:"totalCount"`
	} `json:"items"`
	Fields       rawNodes[rawField]      `json:"fields"`
	Views        rawNodes[rawView]       `json:"views"`
	Workflows    rawNodes[rawWorkflow]   `json:"workflows"`
	Repositories rawNodes[rawRepository] `json:"repositories"`
}

type rawField struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	DataType      string              `json:"dataType"`
	Options       []rawOption         `json:"options"`
	Configuration *rawIterationConfig `json:"configuration"`
}

type rawOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawIterationConfig struct {
	Iterations          []rawIteration `json:"iterations"`
	CompletedIterations []rawIteration `json:"completedIterations"`
}

type rawIteration struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	Duration  int    `json:"duration"`
}

type rawView struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Filter string `json:"filter"`
}

type rawWorkflow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type rawRepository struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type rawUser struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

// discoverResponse is the DiscoverProject answer: one of the two owner
// paths holds the project, the other one is null with a NOT_FOUND error.
type discoverResponse struct {
	Organization *struct {
		ProjectV2 *rawProject `json:"projectV2"`
	} `json:"organization"`
	User *struct {
		ProjectV2 *rawProject `json:"projectV2"`
	} `json:"user"`
	Viewer rawUser `json:"viewer"`
}

func (r discoverResponse) project() *rawProject {
	if r.Organization != nil && r.Organization.ProjectV2 != nil {
		return r.Organization.ProjectV2
	}
	if r.User != nil && r.User.ProjectV2 != nil {
		return r.User.ProjectV2
	}
	return nil
}

// toProject maps the schema. The reference carries the login as GitHub
// spells it, so a differently cased --project still names the same board.
func toProject(raw *rawProject, viewerLogin string, now time.Time) domain.Project {
	p := domain.Project{
		Ref:          domain.ProjectRef{Owner: raw.Owner.Login, Number: raw.Number},
		NodeID:       raw.ID,
		Title:        raw.Title,
		URL:          raw.URL,
		README:       raw.Readme,
		Fields:       make([]domain.Field, 0, len(raw.Fields.Nodes)),
		Views:        make([]domain.View, 0, len(raw.Views.Nodes)),
		Workflows:    make([]domain.Workflow, 0, len(raw.Workflows.Nodes)),
		Repositories: make([]domain.Repository, 0, len(raw.Repositories.Nodes)),
		ItemCount:    raw.Items.TotalCount,
		ViewerRole:   toRole(raw, viewerLogin),
		DiscoveredAt: now,
	}
	for _, f := range raw.Fields.Nodes {
		if f.ID != "" {
			p.Fields = append(p.Fields, toField(f))
		}
	}
	for _, v := range raw.Views.Nodes {
		p.Views = append(p.Views, domain.View{NodeID: v.ID, Number: v.Number, Name: v.Name, Layout: v.Layout, Filter: v.Filter})
	}
	for _, w := range raw.Workflows.Nodes {
		p.Workflows = append(p.Workflows, domain.Workflow{NodeID: w.ID, Name: w.Name, Enabled: w.Enabled})
	}
	for _, r := range raw.Repositories.Nodes {
		p.Repositories = append(p.Repositories, domain.Repository{NodeID: r.ID, Owner: r.Owner.Login, Name: r.Name})
	}
	return p
}

// toRole derives the viewer's role: GitHub exposes no role on ProjectV2,
// so an organization administrator or the owning user is ADMIN, a viewer
// who can update is WRITER, and anyone else who could read it is READER.
func toRole(raw *rawProject, viewerLogin string) domain.Role {
	switch {
	case raw.Owner.TypeName == ownerOrganization && raw.Owner.ViewerCanAdminister,
		raw.Owner.TypeName == ownerUser && viewerLogin != "" && strings.EqualFold(raw.Owner.Login, viewerLogin):
		return domain.RoleAdmin
	case raw.ViewerCanUpdate:
		return domain.RoleWriter
	default:
		return domain.RoleReader
	}
}

func toField(raw rawField) domain.Field {
	f := domain.Field{ID: raw.ID, Name: raw.Name, DataType: domain.DataType(raw.DataType)}
	for _, o := range raw.Options {
		f.Options = append(f.Options, domain.FieldOption{ID: o.ID, Name: o.Name})
	}
	f.Iterations = toIterations(raw.Configuration)
	return f
}

// toIterations lists every iteration in chronological order; GitHub
// answers the completed ones newest first.
func toIterations(cfg *rawIterationConfig) []domain.Iteration {
	if cfg == nil {
		return nil
	}
	out := make([]domain.Iteration, 0, len(cfg.Iterations)+len(cfg.CompletedIterations))
	for _, it := range cfg.CompletedIterations {
		out = append(out, toIteration(it, true))
	}
	for _, it := range cfg.Iterations {
		out = append(out, toIteration(it, false))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

func toIteration(raw rawIteration, completed bool) domain.Iteration {
	start, _ := time.Parse(domain.DateLayout, raw.StartDate)
	return domain.Iteration{ID: raw.ID, Title: raw.Title, Start: start, Duration: raw.Duration, Completed: completed}
}
