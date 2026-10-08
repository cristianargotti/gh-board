package commands

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/cristianargotti/gh-board/internal/mcp"
)

// JSON Schema keywords and types the catalog uses.
const (
	mcpKeyType        = "type"
	mcpKeyDescription = "description"
	mcpKeyDefault     = "default"
	mcpTypeString     = "string"
	mcpTypeArray      = "array"
)

// Arguments every tool shares: the subcommand choice of a runnable
// parent (sprint current|next) and the project override.
const (
	mcpWhichProperty   = "which"
	mcpProjectProperty = "project"
)

// mcpGlobalFlagNames are the global flags a write or plan tool exposes as
// fields; a read tool exposes the first one only.
const mcpGlobalFlagNames = "project dry-run reason expect"

// mcpGlobalFlags lists the global flags a tier exposes as fields: reads
// take the project only; writes and plans also take the dry run, the
// reason and the preconditions.
func mcpGlobalFlags(tier string) []string {
	names := strings.Fields(mcpGlobalFlagNames)
	if tier == mcp.TierRead {
		return names[:1]
	}
	return names
}

// mcpSchema derives the input schema of a command: the declared
// positionals, the subcommand choice, the local flags and the global
// flags of the tier. It also returns the spec the runner needs.
func mcpSchema(cmd *cobra.Command, tier string, globals *pflag.FlagSet) (map[string]any, mcpSpec, error) {
	props := map[string]any{}
	var required []string
	args, err := mcpArgsOf(cmd)
	if err != nil {
		return nil, mcpSpec{}, err
	}
	for i, arg := range args {
		props[arg.name] = mcpArgProperty(arg, i == 0, cmd.ValidArgs)
		required = append(required, arg.name)
	}
	spec := mcpSpec{args: args, which: mcpReadChildren(cmd), flags: map[string]string{}}
	if len(spec.which) > 0 {
		props[mcpWhichProperty] = map[string]any{
			mcpKeyType: mcpTypeString, "enum": spec.which, mcpKeyDefault: spec.which[0],
			mcpKeyDescription: "subcommand to run: " + strings.Join(spec.which, " or "),
		}
	}
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := mcpPropertyName(f.Name)
		props[name], spec.flags[name] = mcpFlagProperty(f), f.Name
		if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok {
			required = append(required, name)
		}
	})
	for _, flag := range mcpGlobalFlags(tier) {
		name := mcpPropertyName(flag)
		props[name], spec.flags[name] = mcpFlagProperty(globals.Lookup(flag)), flag
	}
	schema := map[string]any{mcpKeyType: "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema, spec, nil
}

// mcpPropertyName is the flag name with underscores, the form agents
// write (dry_run).
func mcpPropertyName(flag string) string {
	return strings.ReplaceAll(flag, "-", "_")
}

// mcpArgProperty describes a positional: a string, an array of strings
// when it takes the remaining arguments, with the enum of the command's
// valid arguments on the first one.
func mcpArgProperty(arg mcpArg, first bool, validArgs []string) map[string]any {
	prop := map[string]any{mcpKeyType: mcpTypeString, mcpKeyDescription: arg.description}
	if arg.variadic {
		prop[mcpKeyType] = mcpTypeArray
		prop["items"] = map[string]any{mcpKeyType: mcpTypeString}
		prop["minItems"] = 1
	}
	if first && len(validArgs) > 0 {
		prop["enum"] = validArgs
	}
	return prop
}

// mcpFlagProperty maps a flag to a property by the pflag value type,
// with the usage as description and the default when it says something.
func mcpFlagProperty(f *pflag.Flag) map[string]any {
	prop := map[string]any{mcpKeyDescription: f.Usage}
	switch f.Value.Type() {
	case "bool":
		prop[mcpKeyType] = "boolean"
	case "int", "int64", "uint", "uint64":
		prop[mcpKeyType] = "integer"
		if n, err := strconv.Atoi(f.DefValue); err == nil && n != 0 {
			prop[mcpKeyDefault] = n
		}
	case "float64":
		prop[mcpKeyType] = "number"
	case "stringSlice", "stringArray":
		prop[mcpKeyType] = mcpTypeArray
		prop["items"] = map[string]any{mcpKeyType: mcpTypeString}
	default:
		prop[mcpKeyType] = mcpTypeString
		if f.DefValue != "" {
			prop[mcpKeyDefault] = f.DefValue
		}
	}
	return prop
}
