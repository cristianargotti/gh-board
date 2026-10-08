package main

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// report is the evaluation result: one line per package and file, and the
// entries under the minimum.
type report struct {
	Lines    []string
	Failures []string
}

// String renders the lines followed by the failures.
func (r report) String() string {
	var b strings.Builder
	for _, l := range r.Lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	for _, f := range r.Failures {
		b.WriteString("FAIL ")
		b.WriteString(f)
		b.WriteByte('\n')
	}
	return b.String()
}

type tally struct {
	stmts   int
	covered int
}

func (t tally) percent() float64 {
	if t.stmts == 0 {
		return 0
	}
	return float64(t.covered) * 100 / float64(t.stmts)
}

// evaluate computes per-file and per-package percentages. Sources absent
// from the profile count as zero statements covered out of one, so they
// fail the minimum.
func evaluate(files map[string]*fileCoverage, sources []string, minimum float64) report {
	perFile := make(map[string]tally)
	for name, fc := range files {
		stmts, covered := fc.totals()
		perFile[name] = tally{stmts: stmts, covered: covered}
	}
	for _, src := range sources {
		if _, ok := perFile[src]; !ok {
			perFile[src] = tally{stmts: 1, covered: 0}
		}
	}
	perPackage := make(map[string]tally)
	for name, t := range perFile {
		pkg := path.Dir(name)
		agg := perPackage[pkg]
		agg.stmts += t.stmts
		agg.covered += t.covered
		perPackage[pkg] = agg
	}
	var rep report
	rep.add("package", perPackage, minimum)
	rep.add("file", perFile, minimum)
	return rep
}

// add appends the sorted lines of one scope and records its failures.
func (r *report) add(scope string, tallies map[string]tally, minimum float64) {
	names := make([]string, 0, len(tallies))
	for name := range tallies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t := tallies[name]
		line := fmt.Sprintf("%-7s %6.1f%% %5d/%-5d %s", scope, t.percent(), t.covered, t.stmts, name)
		r.Lines = append(r.Lines, line)
		if t.percent() < minimum {
			r.Failures = append(r.Failures, fmt.Sprintf("%s %s %.1f%%", scope, name, t.percent()))
		}
	}
}
