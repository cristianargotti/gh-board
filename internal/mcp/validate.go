package mcp

import (
	"fmt"
	"math"
	"sort"
)

// validate checks arguments against the subset of JSON Schema the catalog
// uses: an object with typed properties (string, integer, number,
// boolean, array of strings), enums, required names and no additional
// properties. It returns one message per problem, sorted by argument.
func validate(schema map[string]any, args map[string]any) []string {
	properties, _ := schema["properties"].(map[string]any)
	var problems []string
	for _, name := range requiredOf(schema) {
		if _, ok := args[name]; !ok {
			problems = append(problems, fmt.Sprintf("missing argument %q", name))
		}
	}
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		property, ok := properties[name].(map[string]any)
		if !ok {
			problems = append(problems, fmt.Sprintf("unknown argument %q", name))
			continue
		}
		if problem := checkValue(name, property, args[name]); problem != "" {
			problems = append(problems, problem)
		}
	}
	return problems
}

// requiredOf reads the required list of a schema.
func requiredOf(schema map[string]any) []string {
	var out []string
	switch required := schema["required"].(type) {
	case []string:
		out = required
	case []any:
		for _, r := range required {
			if name, ok := r.(string); ok {
				out = append(out, name)
			}
		}
	}
	return out
}

// checkValue validates one argument against its property schema.
func checkValue(name string, property map[string]any, value any) string {
	kind, _ := property["type"].(string)
	switch kind {
	case "string":
		s, ok := value.(string)
		if !ok {
			return fmt.Sprintf("argument %q must be a string", name)
		}
		return checkEnum(name, property, s)
	case "integer":
		if n, ok := value.(float64); !ok || n != math.Trunc(n) {
			return fmt.Sprintf("argument %q must be an integer", name)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Sprintf("argument %q must be a number", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("argument %q must be a boolean", name)
		}
	case "array":
		return checkArray(name, value)
	}
	return ""
}

// checkEnum validates a string against the enum of its property, when
// the property declares one.
func checkEnum(name string, property map[string]any, value string) string {
	allowed, ok := property["enum"].([]string)
	if !ok {
		return ""
	}
	for _, a := range allowed {
		if a == value {
			return ""
		}
	}
	return fmt.Sprintf("argument %q must be one of %v", name, allowed)
}

// checkArray validates an array of strings, the only array type of the
// catalog.
func checkArray(name string, value any) string {
	items, ok := value.([]any)
	if !ok {
		return fmt.Sprintf("argument %q must be an array of strings", name)
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return fmt.Sprintf("argument %q must be an array of strings", name)
		}
	}
	return ""
}
