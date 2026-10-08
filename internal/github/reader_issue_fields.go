package github

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// issueFieldPageSize is the page of the organization issue fields
// connection.
const issueFieldPageSize = 50

// rawOrgIssueField is one node of OrganizationIssueFields.
type rawOrgIssueField struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	DataType string      `json:"dataType"`
	Options  []rawOption `json:"options"`
}

// IssueFields lists the organization issue fields of an owner, by name,
// with their kind and options. A user owner has none, so the NOT_FOUND of
// the organization path answers an empty list.
func (a *Adapter) IssueFields(ctx context.Context, owner string) ([]domain.Field, error) {
	if owner == "" {
		return nil, usage("organization_issue_fields: an owner is required")
	}
	fields := []domain.Field{}
	base := map[string]any{varOwner: owner}
	err := forEachPage(func(after string) (pageInfo, error) {
		var out struct {
			Organization *struct {
				IssueFields struct {
					PageInfo pageInfo           `json:"pageInfo"`
					Nodes    []rawOrgIssueField `json:"nodes"`
				} `json:"issueFields"`
			} `json:"organization"`
		}
		err := a.run(ctx, "organization_issue_fields", pageVariables(base, issueFieldPageSize, after), &out)
		if err != nil && !tolerated(err, "organization") {
			return pageInfo{}, err
		}
		if out.Organization == nil {
			return pageInfo{}, nil
		}
		for _, raw := range out.Organization.IssueFields.Nodes {
			fields = append(fields, toOrgIssueField(raw))
		}
		return out.Organization.IssueFields.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	return fields, nil
}

func toOrgIssueField(raw rawOrgIssueField) domain.Field {
	f := domain.Field{ID: raw.ID, Name: raw.Name, DataType: domain.DataType(raw.DataType)}
	for _, o := range raw.Options {
		f.Options = append(f.Options, domain.FieldOption{ID: o.ID, Name: o.Name})
	}
	return f
}

// IssueFieldID resolves an organization issue field by name, with a "did
// you mean" on mismatch, for an issue that has no value in it yet
// (setIssueFieldValue needs the field id).
func (a *Adapter) IssueFieldID(ctx context.Context, owner, name string) (string, error) {
	if name == "" {
		return "", usage("organization_issue_fields: a field name is required")
	}
	fields, err := a.IssueFields(ctx, owner)
	if err != nil {
		return "", err
	}
	field, err := domain.ResolveField(domain.Project{Fields: fields}, name)
	if err != nil {
		return "", err
	}
	return field.ID, nil
}
