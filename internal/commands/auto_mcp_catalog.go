package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/mcp"
)

// mcpArgsAnnotation names the positional arguments of a command for the
// MCP catalog, one per line as "name: description"; a name ending in
// "..." takes the remaining arguments. Flags need nothing: they are
// introspected. The first positional takes cmd.ValidArgs as its enum.
const mcpArgsAnnotation = "gh-board.args"

// autoMCPExposed is the exact tool set of section 8.4, in catalog order:
// the read verbs, the direct writes and the plan verbs. A dot joins the
// words of a command path; the tool name joins them with an underscore.
// apply, agent, guard, watch, init, milestone and plan are never here.
var autoMCPExposed = "context status attention roadmap item list search epics sprint " +
	"move assign unassign set comment new close tidy digest.post"

// mcpExposure is one entry of the exposed set.
type mcpExposure struct {
	name string
	path []string
}

// mcpExposures parses the exposed set.
func mcpExposures() []mcpExposure {
	var out []mcpExposure
	for _, verb := range strings.Fields(autoMCPExposed) {
		out = append(out, mcpExposure{name: strings.ReplaceAll(verb, ".", "_"), path: strings.Split(verb, ".")})
	}
	return out
}

// mcpArg is one positional argument of a tool.
type mcpArg struct {
	name        string
	description string
	variadic    bool
}

// mcpSpec is what the runner needs to turn arguments back into a command
// line: the path, the positionals in order, the subcommand choices and
// the flag behind every other property.
type mcpSpec struct {
	path  []string
	args  []mcpArg
	which []string
	flags map[string]string
}

// mcpCatalog is the introspected catalog: the tools for the server and
// the specs for the runner, keyed by tool name.
type mcpCatalog struct {
	tools []mcp.Tool
	specs map[string]mcpSpec
}

// autoMCPCatalog introspects the command tree for every exposed tool.
func autoMCPCatalog(root *cobra.Command) (mcpCatalog, error) {
	catalog := mcpCatalog{specs: map[string]mcpSpec{}}
	for _, e := range mcpExposures() {
		cmd, err := mcpFind(root, e.path)
		if err != nil {
			return mcpCatalog{}, err
		}
		tier, err := mcpTier(cmd)
		if err != nil {
			return mcpCatalog{}, err
		}
		schema, spec, err := mcpSchema(cmd, tier, root.PersistentFlags())
		if err != nil {
			return mcpCatalog{}, fmt.Errorf("%s: %w", e.name, err)
		}
		spec.path = e.path
		catalog.specs[e.name] = spec
		catalog.tools = append(catalog.tools, mcp.Tool{
			Name: e.name, Title: cmd.CommandPath(), Description: cmd.Short, Tier: tier, InputSchema: schema,
		})
	}
	return catalog, nil
}

// mcpFind walks the tree along a path of command names.
func mcpFind(root *cobra.Command, path []string) (*cobra.Command, error) {
	cmd := root
	for _, name := range path {
		next := mcpChild(cmd, name)
		if next == nil {
			return nil, fmt.Errorf("mcp catalog: command %q not found under %q", name, cmd.CommandPath())
		}
		cmd = next
	}
	return cmd, nil
}

func mcpChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

// mcpTier maps the command group to the tool tier; a command outside the
// three tiers cannot be a tool.
func mcpTier(cmd *cobra.Command) (string, error) {
	switch cmd.GroupID {
	case GroupRead:
		return mcp.TierRead, nil
	case GroupWrite:
		return mcp.TierWrite, nil
	case GroupPlan:
		return mcp.TierPlan, nil
	}
	return "", fmt.Errorf("mcp catalog: %s has no tier", cmd.CommandPath())
}

// mcpArgsOf reads the positional annotation. A command whose Use line
// shows positionals must declare them, so a verb cannot be exposed with
// an incomplete schema.
func mcpArgsOf(cmd *cobra.Command) ([]mcpArg, error) {
	text := strings.TrimSpace(cmd.Annotations[mcpArgsAnnotation])
	if text == "" {
		if len(strings.Fields(cmd.Use)) > 1 {
			return nil, fmt.Errorf("%s takes positional arguments but declares none for the MCP catalog", cmd.CommandPath())
		}
		return nil, nil
	}
	var args []mcpArg
	for _, line := range strings.Split(text, "\n") {
		name, description, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("%s: positional annotation %q is not name: description", cmd.CommandPath(), line)
		}
		arg := mcpArg{name: strings.TrimSpace(name), description: strings.TrimSpace(description)}
		if rest, variadic := strings.CutSuffix(arg.name, "..."); variadic {
			arg.name, arg.variadic = rest, true
		}
		args = append(args, arg)
	}
	return args, nil
}

// mcpReadChildren lists the read subcommands of a runnable parent, the
// choices of the which argument.
func mcpReadChildren(cmd *cobra.Command) []string {
	if cmd.RunE == nil {
		return nil
	}
	var names []string
	for _, child := range cmd.Commands() {
		if child.GroupID == GroupRead {
			names = append(names, child.Name())
		}
	}
	return names
}
