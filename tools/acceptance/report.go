package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const (
	pass    = "PASS"
	fail    = "FAIL"
	skipped = "SKIP"
)

type row struct{ name, status, detail string }

func (s *suite) record(name string, err error, detail string) {
	status := pass
	if err != nil {
		status, detail = fail, err.Error()
	}
	s.rows = append(s.rows, row{name, status, detail})
}

func report(out io.Writer, rows []row) int {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "CHECK\tRESULT\tDETAIL")
	code := 0
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", r.name, r.status, strings.Join(strings.Fields(r.detail), " "))
		if r.status == fail {
			code = 1
		}
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return code
}
