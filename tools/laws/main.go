// Command laws enforces the text laws of the PCC standards (section 12 of
// the design) on the repository: no em dash or en dash in any text file, no
// Go file over 300 lines, no function over 40 lines (tests included,
// generated files excluded) and no Co-Authored-By or Generated with line in
// the last fifty commit messages. It prints one line per violation and
// exits 1 when any exists, 2 when the tool itself fails.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// Limits of the PCC standards.
const (
	maxFileLines = 300
	maxFuncLines = 40
	commitWindow = "50"
)

type check func(root string) ([]string, error)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

// execute parses the flags, runs the checks and returns the exit code.
func execute(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("laws", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to check")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	violations, err := run(*root)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "laws:", err)
		return 2
	}
	for _, v := range violations {
		_, _ = fmt.Fprintln(stdout, v)
	}
	if len(violations) > 0 {
		_, _ = fmt.Fprintf(stderr, "laws: %d violation(s)\n", len(violations))
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "laws: ok")
	return 0
}

// run executes every check and returns the violations in check order.
func run(root string) ([]string, error) {
	var out []string
	for _, c := range []check{checkDashes, checkGoFiles, checkCommits} {
		v, err := c(root)
		if err != nil {
			return nil, err
		}
		out = append(out, v...)
	}
	return out, nil
}
