package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMCPArgvRendering(t *testing.T) {
	spec := mcpSpec{
		path: []string{"digest", "post"}, which: []string{"current", "next"},
		args:  []mcpArg{{name: "ref"}, {name: "fields", variadic: true}},
		flags: map[string]string{"dry_run": "dry-run", "budget": "budget", "label": "label", "project": "project", "reason": "reason"},
	}
	runner := &autoMCPRunner{project: "acme/7", config: "board.yml"}
	argv, err := runner.argv(spec, map[string]any{
		"which": "next", "ref": "-x", "fields": []any{"a=1", "b=2"}, "dry_run": true, "budget": float64(300),
		"label": []any{"bug", "docs"}, "reason": "why", "project": "acme/9",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "digest post next --json --project=acme/9 --config=board.yml --budget=300 --dry-run --label=bug --label=docs --reason=why -- -x a=1 b=2"
	if got := strings.Join(argv, " "); got != want {
		t.Fatalf("argv = %q\nwant   %q", got, want)
	}
	argv, err = runner.argv(mcpSpec{path: []string{"status"}, flags: map[string]string{"all": "all"}}, map[string]any{"all": false})
	if err != nil || strings.Join(argv, " ") != "status --json --project=acme/7 --config=board.yml" {
		t.Fatalf("argv = %v, %v", argv, err)
	}
}

func TestMCPArgvErrors(t *testing.T) {
	runner := &autoMCPRunner{}
	spec := mcpSpec{path: []string{"x"}, args: []mcpArg{{name: "ref"}}, flags: map[string]string{"n": "n"}}
	cases := []map[string]any{
		{}, {"ref": map[string]any{}}, {"ref": "a", "n": map[string]any{}}, {"ref": "a", "n": []any{map[string]any{}}},
	}
	for _, args := range cases {
		if _, err := runner.argv(spec, args); err == nil {
			t.Errorf("args %v must fail", args)
		}
	}
	if text, err := mcpScalar(true); err != nil || text != "true" {
		t.Fatalf("scalar bool = %q %v", text, err)
	}
}

func TestMCPCatalogErrors(t *testing.T) {
	root := &cobra.Command{Use: "board"}
	bindGlobalFlags(root, &GlobalFlags{})
	root.AddCommand(
		&cobra.Command{Use: "bare <x>", GroupID: GroupRead, RunE: func(*cobra.Command, []string) error { return nil }},
		&cobra.Command{Use: "odd", GroupID: GroupRead, Annotations: map[string]string{mcpArgsAnnotation: "nocolon"}},
		&cobra.Command{Use: "auto", GroupID: GroupAuto},
	)
	for _, tc := range []struct {
		path []string
		want string
	}{
		{[]string{"bare"}, "declares none"},
		{[]string{"odd"}, "name: description"},
		{[]string{"auto"}, "no tier"},
		{[]string{"missing"}, "not found"},
	} {
		cmd, err := mcpFind(root, tc.path)
		if err == nil {
			tier, tierErr := mcpTier(cmd)
			if tierErr == nil {
				_, _, err = mcpSchema(cmd, tier, root.PersistentFlags())
			} else {
				err = tierErr
			}
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: err = %v, want %q", tc.path, err, tc.want)
		}
	}
}

func TestMCPFlagProperties(t *testing.T) {
	cmd := &cobra.Command{Use: "probe"}
	cmd.Flags().Float64("ratio", 0, "a ratio")
	cmd.Flags().Int("count", 0, "a count")
	cmd.Flags().String("secret", "", "hidden")
	cmd.Flags().String("name", "", "a name")
	_ = cmd.Flags().MarkHidden("secret")
	_ = cmd.MarkFlagRequired("name")
	root := &cobra.Command{Use: "board"}
	bindGlobalFlags(root, &GlobalFlags{})
	root.AddCommand(cmd)
	schema, spec, err := mcpSchema(cmd, "write", root.PersistentFlags())
	if err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	if props["ratio"].(map[string]any)["type"] != "number" || props["count"].(map[string]any)["type"] != "integer" {
		t.Fatalf("props = %v", props)
	}
	if _, hidden := props["secret"]; hidden {
		t.Fatal("hidden flag exposed")
	}
	if required := schema["required"].([]string); len(required) != 1 || required[0] != "name" {
		t.Fatalf("required = %v", required)
	}
	if spec.flags["dry_run"] != "dry-run" || props["expect"].(map[string]any)["type"] != "array" {
		t.Fatalf("globals = %v", spec.flags)
	}
}

func TestAutoMCPServerBuildFailure(t *testing.T) {
	previous := autoMCPExposed
	autoMCPExposed += " nope"
	t.Cleanup(func() { autoMCPExposed = previous })
	deps := &Deps{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, In: strings.NewReader("")}
	if err := Execute(t.Context(), []string{"mcp"}, deps); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}
