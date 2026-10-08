package cicheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var pinResults = []struct {
	name, reply string
	valid       bool
}{
	{"matching release commit", "printf '%s\\n' 0123456789012345678901234567890123456789", true},
	{"moved release tag", "printf '%s\\n' 9876543210987654321098765432109876543210", false},
	{"API unavailable", "exit 1", false},
}

func TestPinVerification(t *testing.T) {
	script := pinScript(t)
	for _, tc := range pinResults {
		t.Run(tc.name, func(t *testing.T) {
			dir := pinFixture(t, tc.reply)
			cmd := exec.Command("bash", filepath.ToSlash(script))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+filepath.ToSlash(dir)+string(os.PathListSeparator)+os.Getenv("PATH"))
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v, output=%s", tc.valid, err, output)
			}
			if tc.valid && strings.Count(string(output), "Verified ") != 2 {
				t.Fatalf("expected two action verifications, got %s", output)
			}
		})
	}
}

// pinScript copies the workflow script with LF line endings, so a checkout
// that converted them still runs it under bash.
func pinScript(t *testing.T) string {
	t.Helper()
	source := readText(t, "../../.github/workflows/verify-action-pins.sh")
	script := filepath.Join(t.TempDir(), "verify-action-pins.sh")
	if err := os.WriteFile(script, []byte(source), 0o600); err != nil { //nolint:gosec // a copy of the repository script under the test directory
		t.Fatal(err)
	}
	return script
}

func pinFixture(t *testing.T, reply string) string {
	t.Helper()
	dir := t.TempDir()
	workflows := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := "- uses: example/action@0123456789012345678901234567890123456789 # v1.2.3\n" +
		"- uses: example/action/nested@0123456789012345678901234567890123456789 # v1.2.3\n"
	if err := os.WriteFile(filepath.Join(workflows, "ci.yml"), []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	stub := "#!/usr/bin/env bash\nset -eu\n" +
		"test \"$1\" = api\ntest \"$2\" = repos/example/action/commits/v1.2.3\n" +
		"test \"$3\" = --jq\ntest \"$4\" = .sha\n" + reply + "\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o700); err != nil { //nolint:gosec // Bash must execute this isolated test fixture.
		t.Fatal(err)
	}
	return dir
}
