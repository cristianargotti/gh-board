package mcp

import (
	"strings"
	"testing"
)

func validateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ref":    map[string]any{"type": "string"},
			"kind":   map[string]any{"type": "string", "enum": []string{"task", "epic"}},
			"budget": map[string]any{"type": "integer"},
			"ratio":  map[string]any{"type": "number"},
			"all":    map[string]any{"type": "boolean"},
			"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"any":    map[string]any{},
		},
		"required": []any{"ref", 7},
	}
}

var validateCases = []struct {
	name string
	args map[string]any
	want []string
}{
	{"valid", map[string]any{"ref": "a#1", "kind": "task", "budget": float64(3), "ratio": 0.5, "all": true, "labels": []any{"x"}, "any": 1}, nil},
	{"missing", map[string]any{}, []string{`missing argument "ref"`}},
	{"unknown", map[string]any{"ref": "a", "nope": 1}, []string{`unknown argument "nope"`}},
	{"types", map[string]any{"ref": 1, "kind": "bug", "budget": 1.5, "ratio": "x", "all": "yes", "labels": "x"}, []string{
		`argument "all" must be a boolean`, `argument "budget" must be an integer`, `argument "kind" must be one of [task epic]`,
		`argument "labels" must be an array of strings`, `argument "ratio" must be a number`, `argument "ref" must be a string`,
	}},
	{"array items", map[string]any{"ref": "a", "labels": []any{"x", 2}}, []string{`argument "labels" must be an array of strings`}},
	{"budget object", map[string]any{"ref": "a", "budget": map[string]any{}}, []string{`argument "budget" must be an integer`}},
}

func TestValidate(t *testing.T) {
	for _, tc := range validateCases {
		t.Run(tc.name, func(t *testing.T) {
			got := validate(validateSchema(), tc.args)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("problems = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequiredOfAcceptsStringSlices(t *testing.T) {
	if got := requiredOf(map[string]any{"required": []string{"a", "b"}}); len(got) != 2 {
		t.Fatalf("required = %v", got)
	}
	if got := requiredOf(map[string]any{}); got != nil {
		t.Fatalf("required = %v", got)
	}
}
