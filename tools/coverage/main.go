// Command coverage reads a Go cover profile and fails when any package or
// any file is under the minimum percentage of statements (80 by default).
// A source file that declares functions with statements but is absent from
// the profile counts as zero percent, so an untested package cannot pass
// unnoticed. No file is exempt: every main() in this repository is one
// statement that calls a tested function, and the *_register.go files are
// covered by the root command tests.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

// execute parses the flags, evaluates the profile and returns the exit
// code: 0 when everything meets the minimum, 1 below it, 2 on failure.
func execute(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "coverage.out", "cover profile written by go test")
	root := flags.String("root", ".", "module root holding go.mod")
	minimum := flags.Float64("min", 80, "minimum percentage per package and per file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	rep, err := run(*profile, *root, *minimum)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "coverage:", err)
		return 2
	}
	_, _ = fmt.Fprint(stdout, rep.String())
	if len(rep.Failures) > 0 {
		_, _ = fmt.Fprintf(stderr, "coverage: %d below %.0f%%\n", len(rep.Failures), *minimum)
		return 1
	}
	return 0
}

// run parses the profile, lists the source files that need coverage and
// evaluates both against the minimum.
func run(profilePath, root string, minimum float64) (report, error) {
	f, err := os.Open(profilePath) //nolint:gosec // the profile path is a flag of this tool
	if err != nil {
		return report{}, err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	files, err := parseProfile(f)
	if err != nil {
		return report{}, err
	}
	module, err := modulePath(root)
	if err != nil {
		return report{}, err
	}
	sources, err := sourceFiles(root, module)
	if err != nil {
		return report{}, err
	}
	return evaluate(files, sources, minimum), nil
}
