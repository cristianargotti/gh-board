// Command gh-board is the GitHub CLI extension entry point. It wires the
// adapter, the clock and the outputs into the command tree and maps the
// command error to the exit codes of section 7.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	// IANA zones ship in the binary so that board.yml time zones resolve on
	// Windows machines without a Go installation.
	_ "time/tzdata"

	"github.com/cristianargotti/gh-board/internal/commands"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

// version is set by goreleaser through ldflags.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command tree and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	dirs := config.DefaultPaths()
	repository := releaseRepository()
	adapter := github.New(github.Options{CacheDir: dirs.Cache, ReleaseRepository: repository})
	deps := &commands.Deps{
		Reader:            adapter,
		Writer:            adapter,
		Clock:             github.SystemClock{},
		Dirs:              dirs,
		Out:               stdout,
		Err:               stderr,
		Version:           version,
		ReleaseRepository: repository,
	}
	err := commands.Execute(context.Background(), args, deps)
	if err == nil {
		return int(domain.ExitOK)
	}
	_, _ = fmt.Fprintln(stderr, "gh board:", err)
	return int(domain.CodeOf(err))
}
