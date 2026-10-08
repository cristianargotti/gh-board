package commands

import (
	"context"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var writeStatusConfigCases = []struct {
	name string
	edit func(*domain.Config)
}{
	{"field case", func(c *domain.Config) { c.Capabilities.Status.Field = "flow" }},
	{"done case", func(c *domain.Config) { c.Capabilities.Status.Done = []string{"done"} }},
	{"transition spelling", func(c *domain.Config) { c.Policy.Transitions["Ready"] = []string{"Actve"} }},
	{"WIP spelling", func(c *domain.Config) { c.Policy.WIP["Actve"] = 1 }},
}

func TestWriteStatusConfig(t *testing.T) {
	for _, tc := range writeStatusConfigCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, fake, _ := writeFixture(t)
			tc.edit(deps.Config.Config)
			err := writeTestExecute(context.Background(), []string{"set", "1", "Flow=Done"}, deps)
			if domain.CodeOf(err) != domain.ExitUsage {
				t.Fatalf("%v", err)
			}
			writeAssertNoMutation(t, fake)
		})
	}
}

func TestWriteGenericMove(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	deps.Config.Config.Capabilities = domain.Capabilities{}
	deps.Config.Config.Policy = domain.Policy{}
	fake.project.Fields[0].Name = domain.BuiltinStatusField
	if err := writeTestExecute(context.Background(), []string{"move", "1", "Active"}, deps); err != nil {
		t.Fatal(err)
	}
}

func TestWriteDoneWinsOverOverlappingClasses(t *testing.T) {
	deps, fake, _ := writeFixture(t)
	deps.Config.Config.Capabilities.Status.Active = append(deps.Config.Config.Capabilities.Status.Active, "Done")
	err := writeTestExecute(context.Background(), []string{"set", "1", "Flow=Done"}, deps)
	if domain.CodeOf(err) != domain.ExitPlanRequired {
		t.Fatalf("%v", err)
	}
	writeAssertNoMutation(t, fake)
}
