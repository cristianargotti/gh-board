package cicheck_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var yamlFiles = []string{"../../.goreleaser.yml", "../../.github/workflows/ci.yml", "../../.github/workflows/release.yml", "../../lefthook.yml"}

var configContracts = []struct {
	file string
	path string
	want string
}{
	{"../../.goreleaser.yml", "version", "2"},
	{"../../.goreleaser.yml", "builds.0.main", "./cmd/gh-board"},
	{"../../.goreleaser.yml", "builds.0.env", "CGO_ENABLED=0"},
	{"../../.goreleaser.yml", "builds.0.goos", "darwin,linux,windows"},
	{"../../.goreleaser.yml", "builds.0.goarch", "amd64,arm64"},
	{"../../.goreleaser.yml", "builds.0.flags", "-trimpath"},
	{"../../.goreleaser.yml", "builds.0.ldflags", "-s -w -X main.version={{ .Version }}"},
	{"../../.goreleaser.yml", "archives.0.formats", "binary"},
	{"../../.goreleaser.yml", "checksum.algorithm", "sha256"},
	{"../../.goreleaser.yml", "checksum.name_template", "checksums.txt"},
	{"../../.goreleaser.yml", "sboms.0.artifacts", "binary"},
	{"../../.goreleaser.yml", "sboms.0.cmd", "syft"},
	{"../../.goreleaser.yml", "signs.0.cmd", "cosign"},
	{"../../.goreleaser.yml", "signs.0.artifacts", "checksum"},
	{"../../.goreleaser.yml", "signs.0.args", "sign-blob,--bundle=${signature},--yes,${artifact}"},
	{"../../.goreleaser.yml", "changelog.use", "git"},
	{"../../.github/workflows/ci.yml", "permissions.contents", "read"},
	{"../../.github/workflows/ci.yml", "jobs.ci.strategy.matrix.os", "ubuntu-latest,macos-latest,windows-latest"},
	{"../../.github/workflows/ci.yml", "jobs.ci.env.CGO_ENABLED", "1"},
	{"../../.github/workflows/ci.yml", "jobs.ci.defaults.run.shell", "bash"},
	{"../../.github/workflows/release.yml", "on.push.tags", "v*"},
	{"../../.github/workflows/release.yml", "permissions.contents", "write"},
	{"../../.github/workflows/release.yml", "permissions.id-token", "write"},
	{"../../.github/workflows/release.yml", "permissions.attestations", "write"},
	{"../../.github/workflows/release.yml", "concurrency.cancel-in-progress", "false"},
	{"../../lefthook.yml", "pre-commit.commands.secrets.run", "gitleaks protect --staged --redact"},
	{"../../lefthook.yml", "pre-commit.commands.laws.run", "go run ./tools/laws -root ."},
}

// readText reads a repository file with LF line endings whatever the
// checkout converted them to, so the checks see the committed content.
func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func readYAML(t *testing.T, path string) *yaml.Node {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(readText(t, path)), &node); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := node.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	return node.Content[0]
}

func lookup(t *testing.T, node *yaml.Node, path string) *yaml.Node {
	t.Helper()
	for _, key := range strings.Split(path, ".") {
		node = child(t, node, key)
	}
	return node
}

func child(t *testing.T, node *yaml.Node, key string) *yaml.Node {
	t.Helper()
	if node.Kind == yaml.SequenceNode {
		index, err := strconv.Atoi(key)
		if err == nil && index >= 0 && index < len(node.Content) {
			return node.Content[index]
		}
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
		}
	}
	t.Fatalf("missing YAML key %q at line %d", key, node.Line)
	return nil
}

func scalarList(node *yaml.Node) string {
	if node.Kind != yaml.SequenceNode {
		return node.Value
	}
	values := make([]string, len(node.Content))
	for i, value := range node.Content {
		values[i] = value.Value
	}
	return strings.Join(values, ",")
}

func TestYAMLParses(t *testing.T) {
	for _, path := range yamlFiles {
		t.Run(path, func(t *testing.T) { readYAML(t, path) })
	}
}

func TestConfigurationContracts(t *testing.T) {
	for _, tc := range configContracts {
		t.Run(tc.file+"/"+tc.path, func(t *testing.T) {
			got := scalarList(lookup(t, readYAML(t, tc.file), tc.path))
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
