package render

import (
	"reflect"
	"testing"
)

var fitCases = []struct {
	name      string
	widths    []int
	available int
	want      []int
}{
	{"no limit", []int{10, 20}, 0, []int{10, 20}},
	{"fits", []int{10, 20}, 40, []int{10, 20}},
	{"exact", []int{10, 20}, 32, []int{10, 20}},
	{"shrink widest", []int{10, 20}, 30, []int{10, 18}},
	{"water fills", []int{10, 20}, 22, []int{10, 10}},
	{"rightmost on ties", []int{10, 10}, 21, []int{10, 9}},
	{"minimum reached", []int{10, 10}, 5, []int{4, 4}},
	{"below minimum untouched", []int{2, 3}, 1, []int{2, 3}},
	{"single column", []int{50}, 10, []int{10}},
	{"empty", []int{}, 10, []int{}},
}

func TestFitWidths(t *testing.T) {
	for _, tc := range fitCases {
		t.Run(tc.name, func(t *testing.T) {
			got := fitWidths(tc.widths, tc.available)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("fitWidths(%v, %d) = %v, want %v", tc.widths, tc.available, got, tc.want)
			}
		})
	}
	original := []int{10, 20}
	_ = fitWidths(original, 5)
	if !reflect.DeepEqual(original, []int{10, 20}) {
		t.Fatal("fitWidths must not mutate its input")
	}
}

func TestNaturalWidthsAndPad(t *testing.T) {
	tbl := &Table{Columns: []string{"REF", "TÍTULO"}, Rows: [][]string{{"#1", "ééé"}, {"#1234"}}}
	if got := naturalWidths(tbl); !reflect.DeepEqual(got, []int{5, 6}) {
		t.Fatalf("naturalWidths = %v", got)
	}
	if got := pad("é", 3); got != "é  " {
		t.Fatalf("pad = %q", got)
	}
	if got := pad("long", 2); got != "long" {
		t.Fatalf("pad must not cut: %q", got)
	}
	if got := joinCells([]string{"a", "bcdefgh", ""}, []int{3, 4, 2}); got != "a    b..." {
		t.Fatalf("joinCells = %q", got)
	}
}
