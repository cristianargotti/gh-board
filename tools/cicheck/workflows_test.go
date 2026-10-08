package cicheck_test

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func stepNamed(t *testing.T, steps *yaml.Node, name string) *yaml.Node {
	t.Helper()
	for _, step := range steps.Content {
		var fields map[string]any
		if err := step.Decode(&fields); err != nil {
			t.Fatal(err)
		}
		if fields["name"] == name {
			return step
		}
	}
	t.Fatalf("missing workflow step %q", name)
	return nil
}

func TestActionPins(t *testing.T) {
	for _, file := range []string{"../../.github/workflows/ci.yml", "../../.github/workflows/release.yml"} {
		uses := regexp.MustCompile(`(?m)^.*uses:.*$`).FindAllString(readText(t, file), -1)
		pin := regexp.MustCompile(`uses: [a-zA-Z0-9_./-]+@[0-9a-f]{40} # v[0-9]+\.[0-9]+\.[0-9]+$`)
		if len(uses) == 0 {
			t.Fatalf("no actions in %s", file)
		}
		for _, use := range uses {
			if !pin.MatchString(use) {
				t.Errorf("unpinned action: %s", use)
			}
		}
	}
}

var toolPins = []string{
	"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2",
	"mvdan.cc/gofumpt@v0.11.0",
	"golang.org/x/vuln/cmd/govulncheck@v1.7.0",
	"github.com/zricethezav/gitleaks/v8@v8.30.0",
}

func TestCIPipeline(t *testing.T) {
	config := readYAML(t, "../../.github/workflows/ci.yml")
	lookup(t, config, "on.push")
	lookup(t, config, "on.pull_request")
	steps := lookup(t, config, "jobs.ci.steps")
	tools := lookup(t, stepNamed(t, steps, "Install pinned tools"), "run").Value
	for _, pin := range toolPins {
		if !strings.Contains(tools, "go install "+pin) {
			t.Errorf("missing tool %s", pin)
		}
	}
	if run := lookup(t, stepNamed(t, steps, "Run local pipeline"), "run").Value; run != "make ci" {
		t.Errorf("pipeline = %q", run)
	}
	plugin := lookup(t, config, "jobs.claude-plugin.steps")
	for _, name := range []string{"Validate plugin", "Test plugin"} {
		command := "claude plugin " + strings.ToLower(strings.Fields(name)[0]) + " agents/claude"
		if got := lookup(t, stepNamed(t, plugin, name), "run").Value; got != command {
			t.Errorf("%s: got %q, want %q", name, got, command)
		}
	}
}

func TestSetupGoAndCheckout(t *testing.T) {
	for _, file := range []string{"../../.github/workflows/ci.yml", "../../.github/workflows/release.yml"} {
		jobs := lookup(t, readYAML(t, file), "jobs")
		for i := 1; i < len(jobs.Content); i += 2 {
			steps := lookup(t, jobs.Content[i], "steps")
			checkSetupSteps(t, steps)
		}
	}
}

func checkSetupSteps(t *testing.T, steps *yaml.Node) {
	t.Helper()
	for _, step := range steps.Content {
		var value struct {
			Uses string
			With map[string]string
		}
		if err := step.Decode(&value); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(value.Uses, "actions/setup-go@") {
			if value.With["go-version"] != "1.27.0" || value.With["cache"] != "true" {
				t.Error("Go must use 1.27.0 with caching")
			}
		}
		if strings.HasPrefix(value.Uses, "actions/checkout@") && value.With["persist-credentials"] != "false" {
			t.Error("checkout must not persist credentials")
		}
	}
}
