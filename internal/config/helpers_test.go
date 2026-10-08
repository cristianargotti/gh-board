package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// referenceFile is the example board.yml of section 5.2, kept as testdata.
const referenceFile = "testdata/acme.yml"

// dashes are built from code points so that the laws tool never finds
// them in the sources; the tests assert the outputs carry none.
var dashes = string([]rune{0x2013, 0x2014})

func loadReference(t *testing.T) *domain.Config {
	t.Helper()
	data, err := os.ReadFile(referenceFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Decode(data)
	if err != nil {
		t.Fatalf("Decode(%s): %v", referenceFile, err)
	}
	return cfg
}

func selectField(name string, options ...string) domain.Field {
	f := domain.Field{ID: "F_" + name, Name: name, DataType: domain.DataTypeSingleSelect}
	for _, o := range options {
		f.Options = append(f.Options, domain.FieldOption{ID: "O_" + o, Name: o})
	}
	return f
}

func field(name string, dataType domain.DataType) domain.Field {
	return domain.Field{ID: "F_" + name, Name: name, DataType: dataType}
}

// boardProject is a sanitized discovery of the example board: the fields
// the reference file names, with a few options each.
func boardProject() domain.Project {
	return domain.Project{
		Ref:    domain.ProjectRef{Owner: "acme", Number: 7},
		NodeID: "PVT_kwDOB_sandbox",
		Title:  "Time Produto",
		Fields: []domain.Field{
			field("Title", domain.DataTypeTitle),
			selectField("Status", "BACKLOG", "READY TO DEV", "IN PROGRESS", "TEST / VALIDATION", "DONE"),
			selectField("Épico", "E1 Onboarding", "E2 Alertas"),
			selectField("Frente", "Plataforma", "Dados"),
			field("Sprint", domain.DataTypeIteration),
			field("Estimativa (dias)", domain.DataTypeNumber),
			field("Start date", domain.DataTypeDate),
			field("Target date", domain.DataTypeDate),
			selectField("Situação", "Impedido"),
			selectField("Decisão", "Resolver", "Adiar"),
			selectField("Porta", "Ideia", "Piloto"),
			selectField("Resultado", "Pendente", "Ship"),
		},
		Repositories: []domain.Repository{{NodeID: "R_1", Owner: "acme", Name: "app"}},
		ViewerRole:   domain.RoleAdmin,
	}
}

func fullSchema() config.Schema {
	return config.Schema{
		Project:     boardProject(),
		Labels:      []domain.Label{{Name: "entrada"}, {Name: "urgente"}, {Name: "lab"}},
		IssueTypes:  []domain.IssueType{{ID: "IT_1", Name: "Feature"}, {ID: "IT_2", Name: "Task"}},
		IssueFields: []string{"Start date", "Target date"},
		Members:     []string{"ana", "bruno", "carla"},
	}
}

// writeFiles creates files under root; a value of "DIR" creates a directory.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if content == "DIR" {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertNoDashes(t *testing.T, text string) {
	t.Helper()
	if strings.ContainsAny(text, dashes) {
		t.Fatalf("output carries an em or en dash:\n%s", text)
	}
}
