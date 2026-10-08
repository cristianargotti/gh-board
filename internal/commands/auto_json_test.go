package commands

import (
	"reflect"
	"testing"
)

type autoDeltaCase struct {
	name      string
	base      map[string]any
	fragment  map[string]any
	want      map[string]any
	conflicts []string
}

var autoDeltaCases = []autoDeltaCase{
	{"absent key", map[string]any{}, map[string]any{"a": 1.0}, map[string]any{"a": 1.0}, nil},
	{"present scalar", map[string]any{"a": 1.0}, map[string]any{"a": 1.0}, nil, nil},
	{"other scalar", map[string]any{"a": 1.0}, map[string]any{"a": 2.0}, nil, []string{"a"}},
	{
		"array union",
		map[string]any{"l": []any{"x"}},
		map[string]any{"l": []any{"x", "y", "y"}},
		map[string]any{"l": []any{"y"}},
		nil,
	},
	{
		"nested",
		map[string]any{"h": map[string]any{"e": []any{"x"}}},
		map[string]any{"h": map[string]any{"e": []any{"x", "z"}, "f": []any{"q"}}},
		map[string]any{"h": map[string]any{"e": []any{"z"}, "f": []any{"q"}}},
		nil,
	},
	{"object conflict", map[string]any{"h": "s"}, map[string]any{"h": map[string]any{"e": 1.0}}, nil, []string{"h"}},
	{"array conflict", map[string]any{"l": "s"}, map[string]any{"l": []any{"x"}}, nil, []string{"l"}},
	{
		"nested conflict",
		map[string]any{"h": map[string]any{"v": true}},
		map[string]any{"h": map[string]any{"v": false}},
		nil,
		[]string{"h.v"},
	},
}

func TestAutoJSONDelta(t *testing.T) {
	for _, tc := range autoDeltaCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := autoJSONDelta(tc.base, tc.fragment); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("delta = %v, want %v", got, tc.want)
			}
			if got := autoJSONConflicts(tc.base, tc.fragment, ""); !reflect.DeepEqual(got, tc.conflicts) {
				t.Fatalf("conflicts = %v, want %v", got, tc.conflicts)
			}
		})
	}
}

func TestAutoJSONMergeRoundTrip(t *testing.T) {
	for _, tc := range autoDeltaCases {
		if tc.conflicts != nil || tc.want == nil {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			base := autoJSONClone(tc.base).(map[string]any)
			merged := autoJSONMerge(base, tc.want)
			if autoJSONDelta(merged, tc.fragment) != nil {
				t.Fatalf("merge left a delta: %v", merged)
			}
			autoJSONUnmerge(merged, tc.want)
			if !reflect.DeepEqual(merged, tc.base) {
				t.Fatalf("unmerge = %v, want %v", merged, tc.base)
			}
		})
	}
}

func TestAutoJSONUnmergeDropsEmptyContainers(t *testing.T) {
	base := map[string]any{"hooks": map[string]any{"e": []any{"x"}}, "version": 1.0, "keep": "me"}
	autoJSONUnmerge(base, map[string]any{"hooks": map[string]any{"e": []any{"x"}}, "version": 1.0, "gone": true})
	if !reflect.DeepEqual(base, map[string]any{"keep": "me"}) {
		t.Fatalf("base = %v", base)
	}
	merged := autoJSONMerge(map[string]any{"a": "old"}, map[string]any{"a": "new", "b": []any{1.0}})
	if merged["a"] != "new" || !reflect.DeepEqual(merged["b"], []any{1.0}) {
		t.Fatalf("merge = %v", merged)
	}
}

var autoObjectCases = []struct {
	name string
	data string
	ok   bool
	keys int
}{
	{"empty", "", true, 0},
	{"blank", " \n", true, 0},
	{"object", `{"a":1,"b":[1]}`, true, 2},
	{"null", "null", false, 0},
	{"array", "[1]", false, 0},
	{"broken", "{bad", false, 0},
}

func TestAutoJSONObject(t *testing.T) {
	for _, tc := range autoObjectCases {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := autoJSONObject([]byte(tc.data), "f.json")
			if (err == nil) != tc.ok || len(obj) != tc.keys {
				t.Fatalf("obj = %v, err = %v", obj, err)
			}
		})
	}
}

func TestAutoJSONEncodeAndFragment(t *testing.T) {
	data, err := autoJSONEncode(map[string]any{"b": 1.0, "a": "<x>"})
	if err != nil || string(data) != "{\n  \"a\": \"<x>\",\n  \"b\": 1\n}\n" {
		t.Fatalf("encode = %q, %v", data, err)
	}
	frag, err := autoFragment(struct {
		Deny []string `json:"deny"`
	}{Deny: []string{"a"}})
	if err != nil || !reflect.DeepEqual(frag, map[string]any{"deny": []any{"a"}}) {
		t.Fatalf("fragment = %v, %v", frag, err)
	}
	if _, err := autoFragment(make(chan int)); err == nil {
		t.Fatal("a channel must not encode")
	}
	if _, err := autoFragment([]int{1}); err == nil {
		t.Fatal("an array is not an object")
	}
	if _, err := autoJSONEncode(map[string]any{"c": make(chan int)}); err == nil {
		t.Fatal("a channel must not encode")
	}
}
