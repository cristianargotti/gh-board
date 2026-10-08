package commands

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/mcp"
)

// autoMCPRunner turns a validated tool call into a command line and runs
// it in process against the same dependencies as the command line.
type autoMCPRunner struct {
	deps  *Deps
	specs map[string]mcpSpec
	// project and config are the global flags of the mcp invocation.
	project string
	config  string
}

// run executes one call. Every call gets its own outputs, an empty
// input and fresh flags, so calls never see each other's state; the
// reader, the writer, the clock and the directories are shared.
func (r *autoMCPRunner) run(ctx context.Context, tool mcp.Tool, args map[string]any) mcp.Outcome {
	spec, ok := r.specs[tool.Name]
	if !ok {
		return mcp.Outcome{Code: domain.ExitUsage, Message: "tool " + tool.Name + " is not in the catalog"}
	}
	argv, err := r.argv(spec, args)
	if err != nil {
		return mcp.Outcome{Code: domain.ExitUsage, Message: err.Error()}
	}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	call := *r.deps
	call.Out, call.Err, call.In, call.Flags = out, errOut, strings.NewReader(""), GlobalFlags{}
	err = Execute(ctx, argv, &call)
	outcome := mcp.Outcome{Code: domain.CodeOf(err), Output: out.Bytes(), Stderr: errOut.String()}
	if err != nil {
		outcome.Message = err.Error()
	}
	return outcome
}

// argv builds the command line: the path and the subcommand choice, the
// JSON and project flags, the other flags as --name=value, then the
// positionals after a double dash so a value that starts with a dash is
// never read as a flag.
func (r *autoMCPRunner) argv(spec mcpSpec, args map[string]any) ([]string, error) {
	argv := append([]string(nil), spec.path...)
	if which, ok := args[mcpWhichProperty].(string); ok && which != "" {
		argv = append(argv, which)
	}
	argv = append(argv, "--json")
	project := r.project
	if value, ok := args[mcpProjectProperty].(string); ok && value != "" {
		project = value
	}
	if project != "" {
		argv = append(argv, "--project="+project)
	}
	if r.config != "" {
		argv = append(argv, "--config="+r.config)
	}
	flags, err := mcpFlagArgs(spec, args)
	if err != nil {
		return nil, err
	}
	argv = append(argv, flags...)
	positionals, err := mcpPositionalArgs(spec, args)
	if err != nil {
		return nil, err
	}
	if len(positionals) > 0 {
		argv = append(argv, "--")
		argv = append(argv, positionals...)
	}
	return argv, nil
}

// mcpFlagArgs renders the flag properties in name order, so the same
// call always produces the same command line.
func mcpFlagArgs(spec mcpSpec, args map[string]any) ([]string, error) {
	names := make([]string, 0, len(args))
	for name := range args {
		if name != mcpProjectProperty && spec.flags[name] != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var out []string
	for _, name := range names {
		rendered, err := mcpFlagValues(spec.flags[name], args[name])
		if err != nil {
			return nil, err
		}
		out = append(out, rendered...)
	}
	return out, nil
}

// mcpFlagValues renders one property as flag arguments: a true boolean is
// the bare flag, a false one nothing, an array one flag per entry.
func mcpFlagValues(flag string, value any) ([]string, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return []string{"--" + flag}, nil
		}
		return nil, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text, err := mcpScalar(item)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", flag, err)
			}
			out = append(out, "--"+flag+"="+text)
		}
		return out, nil
	}
	text, err := mcpScalar(value)
	if err != nil {
		return nil, fmt.Errorf("--%s: %w", flag, err)
	}
	return []string{"--" + flag + "=" + text}, nil
}

// mcpPositionalArgs renders the positionals in the declared order; a
// variadic one contributes every entry of its array.
func mcpPositionalArgs(spec mcpSpec, args map[string]any) ([]string, error) {
	var out []string
	for _, arg := range spec.args {
		value, ok := args[arg.name]
		if !ok {
			return nil, fmt.Errorf("missing argument %q", arg.name)
		}
		items := []any{value}
		if list, isList := value.([]any); isList && arg.variadic {
			items = list
		}
		for _, item := range items {
			text, err := mcpScalar(item)
			if err != nil {
				return nil, fmt.Errorf("argument %q: %w", arg.name, err)
			}
			out = append(out, text)
		}
	}
	return out, nil
}

// mcpScalar renders a JSON scalar as a command line value.
func mcpScalar(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	}
	return "", fmt.Errorf("value %v is not a string, a number or a boolean", value)
}
