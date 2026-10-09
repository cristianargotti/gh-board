package commands

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var autoDiffCases = []struct {
	name, before, after, want string
}{
	{"equal", "a\n", "a\n", ""},
	{"change line", "a\nb\nc\n", "a\nx\nc\n", "--- f\n+++ f\n@@ -1,3 +1,3 @@\n a\n-b\n+x\n c\n"},
	{"append", "", "a\n", "--- f\n+++ f\n@@ -1,0 +1,1 @@\n+a\n"},
	{"remove all", "a\n", "", "--- f\n+++ f\n@@ -1,1 +1,0 @@\n-a\n"},
	{"no final newline", "a", "a\nb", "--- f\n+++ f\n@@ -1,1 +1,2 @@\n a\n+b\n"},
}

func TestAutoDiff(t *testing.T) {
	for _, tc := range autoDiffCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := autoDiff("f", []byte(tc.before), []byte(tc.after)); got != tc.want {
				t.Fatalf("diff =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestAutoDiffHunks(t *testing.T) {
	var before, after []string
	for i := 1; i <= 20; i++ {
		line := fmt.Sprint(i)
		before = append(before, line)
		switch i {
		case 2:
			line = "x"
		case 18:
			line = "y"
		}
		after = append(after, line)
	}
	got := autoDiff("f", []byte(strings.Join(before, "\n")+"\n"), []byte(strings.Join(after, "\n")+"\n"))
	for _, want := range []string{"@@ -1,5 +1,5 @@\n", "@@ -15,6 +15,6 @@\n", "-2\n+x\n", "-18\n+y\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diff lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "@@ -") != 2 {
		t.Fatalf("hunks:\n%s", got)
	}
}

func TestAutoDiffHelpers(t *testing.T) {
	ops := autoReplaceOps([]string{"a"}, []string{"b", "c"})
	want := []autoDiffOp{{'-', "a"}, {'+', "b"}, {'+', "c"}}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("replace ops = %v", ops)
	}
	if hunks := autoDiffHunks([]autoDiffOp{{' ', "a"}}); hunks != nil {
		t.Fatalf("hunks of an unchanged file = %v", hunks)
	}
	cases := map[string][]string{"": nil, "a": {"a"}, "a\n": {"a"}, "\n": {""}, "a\n\nb\n": {"a", "", "b"}}
	for in, want := range cases {
		if got := autoSplitLines([]byte(in)); !reflect.DeepEqual(got, want) {
			t.Errorf("split %q = %q, want %q", in, got, want)
		}
	}
	if ops := autoDiffOps(nil, nil); len(ops) != 0 {
		t.Fatalf("ops of nothing = %v", ops)
	}
}

func TestAutoDiffFallsBackAboveTheLimit(t *testing.T) {
	a, b := make([]string, 2001), make([]string, 2000)
	if table := autoLCSTable(a, b); table != nil {
		t.Fatalf("table above the limit = %d rows, want nil", len(table))
	}
	if ops := autoDiffOps(a, b); len(ops) != len(a)+len(b) {
		t.Fatalf("fallback ops = %d, want %d", len(ops), len(a)+len(b))
	}
	if table := autoLCSTable([]string{"a"}, []string{"a"}); table == nil || table[0][0] != 1 {
		t.Fatalf("table within the limit = %v", table)
	}
}
