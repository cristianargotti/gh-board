package assets

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"regexp"
	"testing"
)

var installerAssets = []string{
	"agents/SKILL.md",
	"agents/claude/.claude-plugin/plugin.json",
	"agents/claude/skills/gh-board/SKILL.md",
	"agents/claude/hooks/hooks.json",
	"agents/claude/hooks/register.tsx",
	"agents/claude/types/index.d.ts",
	"agents/cursor/hooks.json",
	"agents/cursor/gh-board.mdc",
	"agents/codex/hooks.json",
	"agents/codex/gh-board.rules",
	"agents/codex/AGENTS.md",
	"templates/board.yml",
	"templates/README.md",
	"templates/forms/config.yml",
	"templates/forms/epico.yml",
	"templates/forms/tarefa.yml",
	"templates/forms/oportunidade.yml",
	"templates/forms/erro.yml",
	"templates/forms/regra.yml",
	"templates/forms/sinal.yml",
	"templates/forms/experimento.yml",
}

var assetRoots = []string{"agents", "templates"}

var shellMatcherCases = []struct {
	tool string
	want bool
}{
	{"Bash", true},
	{"apply_patch", false},
	{"Read", false},
	{"mcp__Bash__run", false},
	{"update_plan", false},
}

func TestCodexHookOnlyMatchesShell(t *testing.T) {
	var document struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(readAsset(t, "agents/codex/hooks.json"), &document); err != nil {
		t.Fatal(err)
	}
	groups := document.Hooks["PreToolUse"]
	if len(groups) != 1 {
		t.Fatalf("got %d PreToolUse groups, want one shell guard", len(groups))
	}
	matcher, err := regexp.Compile(groups[0].Matcher)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range shellMatcherCases {
		if got := matcher.MatchString(tc.tool); got != tc.want {
			t.Errorf("matcher(%q) = %v, want %v", tc.tool, got, tc.want)
		}
	}
}

func TestSkillHasOneSource(t *testing.T) {
	source := readAsset(t, "agents/SKILL.md")
	plugin := readAsset(t, "agents/claude/skills/gh-board/SKILL.md")
	if !bytes.Equal(source, plugin) {
		t.Fatal("copy agents/SKILL.md to agents/claude/skills/gh-board/SKILL.md after editing the source")
	}
}

func TestInstallerAssetsExist(t *testing.T) {
	for _, name := range installerAssets {
		t.Run(name, func(t *testing.T) {
			if len(readAsset(t, name)) == 0 {
				t.Fatal("installer asset is empty")
			}
		})
	}
}

func TestAllSourceAssetsAreEmbedded(t *testing.T) {
	source := os.DirFS(".")
	for _, root := range assetRoots {
		err := fs.WalkDir(source, root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			want, err := fs.ReadFile(source, path)
			if err != nil {
				return err
			}
			if !bytes.Equal(readAsset(t, path), want) {
				t.Errorf("embedded asset %s differs from its source", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func readAsset(t *testing.T, name string) []byte {
	t.Helper()
	data, err := FS.ReadFile(name)
	if err != nil {
		t.Fatalf("installer asset %s is not embedded: %v", name, err)
	}
	return data
}
