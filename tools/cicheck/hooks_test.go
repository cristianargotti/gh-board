package cicheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var commitMessages = []struct {
	name, message string
	valid         bool
}{
	{"simple", "feat: add board context\n", true},
	{"scope", "fix(api): handle pagination\n", true},
	{"breaking", "feat(cli)!: change plan format\n", true},
	{"body", "fix: preserve fields\n\nKeep the existing field values.\n", true},
	{"comments", "ci: check releases\n\n# Editor hint\n", true},
	{"crlf", "ci: check releases\r\n\r\nValidate all targets.\r\n", true},
	{"empty", "", false},
	{"comment only", "# Empty message\n", false},
	{"blank subject", "\nfeat: add context\n", false},
	{"type", "feature: add context\n", false},
	{"description", "fix: \n", false},
	{"uppercase", "fix: Preserve fields\n", false},
	{"body separator", "fix: preserve fields\nKeep values.\n", false},
	{"coauthor", "fix: preserve fields\n\nCo-authored-by: Example <example@example.test>\n", false},
	{"signoff", "fix: preserve fields\n\nSigned-off-by: Example <example@example.test>\n", false},
	{"custom trailer", "fix: preserve fields\n\nReviewed-by: Example\n", false},
	{"reference footer", "fix: preserve fields\n\nRefs #123\n", false},
	{"breaking footer", "feat!: change fields\n\nBREAKING CHANGE: new format\n", false},
	{"generated", "fix: preserve fields\n\nGenerated with a tool\n", false},
	{"non ascii", "fix: preserve " + string(rune(0xe9)) + "\n", false},
	{"dash", "fix: preserve " + string(rune(0x2014)) + " fields\n", false},
	{"control", "fix: preserve fields\x1b\n", false},
}

func TestCommitMessages(t *testing.T) {
	config := readYAML(t, "../../lefthook.yml")
	script := lookup(t, config, "commit-msg.commands.conventional.run").Value
	script = strings.ReplaceAll(script, "{1}", "$1")
	for _, tc := range commitMessages {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "message with spaces")
			if err := os.WriteFile(file, []byte(tc.message), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("bash", "-c", script, "commit-msg", filepath.ToSlash(file)).CombinedOutput()
			if (err == nil) != tc.valid {
				t.Errorf("valid=%v, error=%v, output=%s", tc.valid, err, output)
			}
		})
	}
}

var formatterCases = []struct {
	name, source string
	valid        bool
}{
	{"formatted", "package fixture\n\nfunc ok() {}\n", true},
	{"unformatted", "package fixture\nfunc ok( ) {}\n", false},
	{"invalid", "package fixture\nfunc {\n", false},
}

func TestFormatHook(t *testing.T) {
	config := readYAML(t, "../../lefthook.yml")
	script := lookup(t, config, "pre-commit.commands.format.run").Value
	script = strings.ReplaceAll(script, "{staged_files}", `"$1"`)
	for _, tc := range formatterCases {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "source with spaces.go")
			if err := os.WriteFile(file, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("bash", "-c", script, "pre-commit", filepath.ToSlash(file)).CombinedOutput()
			if (err == nil) != tc.valid {
				t.Errorf("valid=%v, error=%v, output=%s", tc.valid, err, output)
			}
			data, err := os.ReadFile(file)
			if err != nil || string(data) != tc.source {
				t.Fatalf("hook changed the working tree: %v", err)
			}
		})
	}
}
