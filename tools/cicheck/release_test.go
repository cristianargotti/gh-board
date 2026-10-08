package cicheck_test

import (
	"path"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

var targets = []struct {
	os, arch, extension string
}{
	{"darwin", "amd64", ""},
	{"darwin", "arm64", ""},
	{"linux", "amd64", ""},
	{"linux", "arm64", ""},
	{"windows", "amd64", ".exe"},
	{"windows", "arm64", ".exe"},
}

var changes = []struct {
	message, group string
}{
	{"feat: add board context", "Features"},
	{"fix(api): preserve pagination", "Bug fixes"},
	{"feat(cli)!: change plan format", "Breaking changes"},
	{"refactor!: replace schema format", "Breaking changes"},
	{"perf: reduce allocations", "Performance"},
	{"docs(setup): explain authentication", "Documentation"},
	{"ci: validate release configuration", "Maintenance"},
}

func renderName(t *testing.T, source, goos, arch string) string {
	t.Helper()
	tmpl, err := template.New("asset").Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	data := map[string]string{"Version": "1.2.3", "Os": goos, "Arch": arch}
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestExtensionAssetNames(t *testing.T) {
	config := readYAML(t, "../../.goreleaser.yml")
	for _, tc := range targets {
		t.Run(tc.os+"/"+tc.arch, func(t *testing.T) {
			want := "gh-board_1.2.3_" + tc.os + "-" + tc.arch + tc.extension
			for _, key := range []string{"builds.0.binary", "archives.0.name_template"} {
				source := lookup(t, config, key).Value
				got := renderName(t, source, tc.os, tc.arch) + tc.extension
				if got != want {
					t.Errorf("%s: got %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestAttestationIncludesEveryBinary(t *testing.T) {
	release := readYAML(t, "../../.github/workflows/release.yml")
	steps := lookup(t, release, "jobs.release.steps")
	attest := stepNamed(t, steps, "Attest binaries")
	globs := strings.Fields(lookup(t, attest, "with.subject-path").Value)
	for _, tc := range targets {
		name := "dist/gh-board_" + tc.os + "_" + tc.arch + "_v1/gh-board_1.2.3_" + tc.os + "-" + tc.arch + tc.extension
		assertMatches(t, globs, name, true)
	}
	for _, name := range []string{"dist/checksums.txt", "dist/checksums.txt.sigstore.json", "dist/gh-board_1.2.3_linux-amd64.sbom.spdx.json"} {
		assertMatches(t, globs, name, false)
	}
}

func assertMatches(t *testing.T, globs []string, name string, want bool) {
	t.Helper()
	matched := false
	for _, glob := range globs {
		ok, err := path.Match(glob, name)
		if err != nil {
			t.Fatal(err)
		}
		matched = matched || ok
	}
	if matched != want {
		t.Errorf("attestation match for %s = %v, want %v", name, matched, want)
	}
}

func TestConventionalReleaseNotes(t *testing.T) {
	groups := lookup(t, readYAML(t, "../../.goreleaser.yml"), "changelog.groups")
	for _, tc := range changes {
		t.Run(tc.message, func(t *testing.T) {
			if got := changeGroup(t, groups, tc.message); got != tc.group {
				t.Errorf("got %q, want %q", got, tc.group)
			}
		})
	}
}

func changeGroup(t *testing.T, groups *yaml.Node, message string) string {
	t.Helper()
	for _, group := range groups.Content {
		var value struct{ Title, Regexp string }
		if err := group.Decode(&value); err != nil {
			t.Fatal(err)
		}
		if value.Regexp == "" || regexp.MustCompile(value.Regexp).MatchString(message) {
			return value.Title
		}
	}
	return ""
}
