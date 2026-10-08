package commands

import (
	"fmt"
	"path"

	assets "github.com/cristianargotti/gh-board"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// Embedded template assets init renders (section 10).
const (
	planAssetTemplate = "templates/board.yml"
	planAssetForms    = "templates/forms"
)

// planFormNames are the issue forms of the template plus the selector
// configuration, written under .github/ISSUE_TEMPLATE in this order.
var planFormNames = []string{"config", "epico", "tarefa", "oportunidade", "erro", "regra", "sinal", "experimento"}

// planReferenceConfig decodes the reference board.yml the forms were
// written against.
func planReferenceConfig() (*domain.Config, error) {
	data, err := assets.FS.ReadFile(planAssetTemplate)
	if err != nil {
		return nil, fmt.Errorf("embedded %s: %w", planAssetTemplate, err)
	}
	return config.Decode(data)
}

// planForms renders the embedded PT-BR forms for the team: the reference
// names become the names of the same board.yml keys, the board link points
// at the project, and a form whose issue type the organization lacks loses
// its type line (templates/README.md lists the pairs).
func planForms(cfg *domain.Config, project domain.Project, types []domain.IssueType) (map[string][]byte, error) {
	reference, err := planReferenceConfig()
	if err != nil {
		return nil, err
	}
	subs := config.FormSubstitutionsFor(reference, cfg, project, types)
	result := make(map[string][]byte, len(planFormNames))
	for _, name := range planFormNames {
		data, err := assets.FS.ReadFile(path.Join(planAssetForms, name+".yml"))
		if err != nil {
			return nil, fmt.Errorf("embedded form %s: %w", name, err)
		}
		rendered, err := config.RenderForm(name, data, subs)
		if err != nil {
			return nil, err
		}
		result[name] = rendered
	}
	return result, nil
}
