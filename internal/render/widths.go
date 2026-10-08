package render

// minColumnWidth is the narrowest a fitted column gets: one rune and the
// ellipsis, so a truncated cell still shows it was truncated.
const minColumnWidth = len(Ellipsis) + 1

// naturalWidths returns the widest cell of every column, header included.
func naturalWidths(t *Table) []int {
	widths := make([]int, len(t.Columns))
	for i := range widths {
		widths[i] = t.Width(i)
	}
	return widths
}

// fitWidths shrinks the widest column one rune at a time until the row
// fits the available width or every column reached the minimum, so a
// narrow terminal keeps one line per row. Ties go to the rightmost column,
// which keeps short reference columns intact longer. Zero or less means no
// limit. The input is never mutated.
func fitWidths(widths []int, available int) []int {
	out := make([]int, len(widths))
	copy(out, widths)
	if available <= 0 || len(out) == 0 {
		return out
	}
	for rowWidth(out) > available {
		i := widest(out)
		if out[i] <= minColumnWidth {
			break
		}
		out[i]--
	}
	return out
}

// rowWidth is the width of a row with the gaps between columns.
func rowWidth(widths []int) int {
	n := len(gap) * (len(widths) - 1)
	for _, w := range widths {
		n += w
	}
	return n
}

// widest returns the index of the widest column, the rightmost on ties.
func widest(widths []int) int {
	best := 0
	for i, w := range widths {
		if w >= widths[best] {
			best = i
		}
	}
	return best
}
