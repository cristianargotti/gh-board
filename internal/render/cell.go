package render

import (
	"strings"
	"unicode/utf8"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// truncateCell cuts a cell to width runes with the ellipsis. The table
// format carries no data delimiters, so every cell is cut the same way.
func truncateCell(cell string, width int) string {
	if width <= 0 || utf8.RuneCountInString(cell) <= width {
		return cell
	}
	return cut(cell, width)
}

// cut truncates s to limit runes with the ellipsis.
func cut(s string, limit int) string {
	out, _ := domain.Truncate(s, limit)
	return out
}

// pad right-pads s with spaces to width runes.
func pad(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}
