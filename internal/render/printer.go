package render

import (
	"io"
	"strings"
)

// printer writes lines and keeps the first write error, so a renderer
// stays linear and checks once at the end.
type printer struct {
	w   io.Writer
	err error
}

// line writes the parts joined, followed by a newline.
func (p *printer) line(parts ...string) {
	if p.err != nil {
		return
	}
	_, p.err = io.WriteString(p.w, strings.Join(parts, "")+"\n")
}

// blocks separates the blocks of a text format with one blank line, never
// before the first block.
type blocks struct {
	p *printer
	n int
}

// start announces a new block and writes the separator when needed.
func (b *blocks) start() {
	if b.n > 0 {
		b.p.line()
	}
	b.n++
}
