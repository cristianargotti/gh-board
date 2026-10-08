package commands

import (
	"context"
	"testing"
)

var parentDefaultCases = []struct {
	name     string
	args     []string
	explicit []string
}{
	{"sprint table", []string{"sprint"}, []string{"sprint", "current"}},
	{"sprint json", []string{"sprint", "--json"}, []string{"sprint", "current", "--json"}},
	{"digest table", []string{"digest"}, []string{"digest", "print"}},
	{"digest week", []string{"digest", "--week", "2026-W40", "--json"}, []string{"digest", "print", "--week", "2026-W40", "--json"}},
}

func TestSharedParentDefaults(t *testing.T) {
	for _, tc := range parentDefaultCases {
		t.Run(tc.name, func(t *testing.T) {
			deps, out := readTestDeps(t, readNewFakeReader())
			if err := Execute(context.Background(), tc.args, deps); err != nil {
				t.Fatal(err)
			}
			got := out.String()
			out.Reset()
			if err := Execute(context.Background(), tc.explicit, deps); err != nil {
				t.Fatal(err)
			}
			if got == "" || got != out.String() {
				t.Fatalf("implicit and explicit output differ:\n%s\n%s", got, out)
			}
		})
	}
}
