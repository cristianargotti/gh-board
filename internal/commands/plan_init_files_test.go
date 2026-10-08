package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestPlanInitLocalFiles(t *testing.T) {
	for _, forms := range []bool{false, true} {
		t.Run(map[bool]string{false: "draft", true: "forms"}[forms], func(t *testing.T) {
			planTestInitFilesCase(t, forms)
		})
	}
}

func planTestInitFilesCase(t *testing.T, forms bool) {
	t.Helper()
	deps, reader, _, _ := planTestDeps(t)
	cfg := deps.Config.Config
	cfg.Capabilities.Triage = &domain.TriageCapability{Label: "entrada"}
	cfg.Capabilities.Lane = &domain.FieldCapability{Field: "Area"}
	cfg.Capabilities.Sprint = &domain.FieldCapability{Field: "Cycle"}
	cfg.Capabilities.Estimate = &domain.FieldCapability{Field: "Missing"}
	reader.project.Fields = []domain.Field{{Name: "Area", DataType: domain.DataTypeSingleSelect, Options: []domain.FieldOption{{Name: "Produto"}}}, {Name: "Cycle", DataType: domain.DataTypeIteration}}
	local := planInitLocal{Output: filepath.Join(t.TempDir(), "board.yml"), Forms: forms, Config: cfg}
	if err := planInitWrite(local, reader.project); err != nil {
		t.Fatal(err)
	}
	if err := planInitWrite(local, reader.project); err != nil {
		t.Fatal("retry:", err)
	}
	data, err := os.ReadFile(local.Output)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := config.Decode(data)
	if err != nil || draft.Repository != cfg.Repository || draft.Project != reader.project.Ref {
		t.Fatalf("draft = %+v, %v", draft, err)
	}
	if forms {
		planTestForms(t, filepath.Dir(local.Output))
	}
}

func planTestForms(t *testing.T, root string) {
	t.Helper()
	for _, name := range planFormNames {
		data, err := os.ReadFile(filepath.Join(root, ".github", "ISSUE_TEMPLATE", name+".yml"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "Frente") || strings.Contains(text, "\ntype: ") {
			t.Fatalf("form %s keeps reference names or a type the organization lacks:\n%s", name, text)
		}
		if name != "config" && !strings.Contains(text, "name: ") {
			t.Fatalf("form %s has no name:\n%s", name, text)
		}
	}
	tarefa, err := os.ReadFile(filepath.Join(root, ".github", "ISSUE_TEMPLATE", "tarefa.yml"))
	if err != nil || !strings.Contains(string(tarefa), "label: Area") {
		t.Fatalf("tarefa form must carry the team lane field: %v\n%s", err, tarefa)
	}
	regra, err := os.ReadFile(filepath.Join(root, ".github", "ISSUE_TEMPLATE", "regra.yml"))
	if err != nil || !strings.Contains(string(regra), `labels: ["entrada", "regra"]`) {
		t.Fatalf("regra form keeps the triage label: %v\n%s", err, regra)
	}
}

func TestPlanLocalWriteProtection(t *testing.T) {
	for _, kind := range []string{"changed", "directory", "parent-file", "fresh"} {
		t.Run(kind, func(t *testing.T) {
			planTestProtectionCase(t, kind)
		})
	}
}

func planTestProtectionCase(t *testing.T, kind string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "board.yml")
	switch kind {
	case "changed":
		if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	case "parent-file":
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		path = filepath.Join(path, "child")
	}
	err := planLocalWrite(path, []byte("draft"))
	if (err == nil) != (kind == "fresh") {
		t.Fatalf("result = %v", err)
	}
}

func TestPlanInitDraftErrors(t *testing.T) {
	for _, kind := range []string{"no-config", "no-project", "bad-path"} {
		t.Run(kind, func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			local := planInitLocal{Output: filepath.Join(t.TempDir(), "board.yml"), Config: deps.Config.Config}
			switch kind {
			case "no-config":
				local.Config = nil
			case "no-project":
				reader.project.Ref = domain.ProjectRef{}
			case "bad-path":
				local.Output = string(rune(0))
			}
			if err := planInitWrite(local, reader.project); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}

func TestPlanFormsRefuseNamesThatBreakYAML(t *testing.T) {
	deps, reader, _, _ := planTestDeps(t)
	cfg := deps.Config.Config
	cfg.Capabilities.Lane = &domain.FieldCapability{Field: `Area "x": [`}
	if _, err := planForms(cfg, reader.project, nil); domain.CodeOf(err) != domain.ExitUsage {
		t.Fatalf("a lane name that breaks the YAML must be refused: %v", err)
	}
	cfg.Capabilities.Lane = nil
	forms, err := planForms(cfg, reader.project, []domain.IssueType{{Name: "Task"}})
	if err != nil || len(forms) != len(planFormNames) || !strings.Contains(string(forms["tarefa"]), "\ntype: Task\n") {
		t.Fatalf("forms: %v, %d rendered", err, len(forms))
	}
}
