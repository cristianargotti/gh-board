// Package commands holds one use case per file, each returning a result
// the renderer prints. root.go wires the cobra root and the global flags;
// the four *_register.go files register the commands of each tier.
package commands

import (
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// GlobalFlags are the flags of section 7 shared by every command.
type GlobalFlags struct {
	// Project is --project owner/number and overrides any file.
	Project string
	// Config is --config path, first in the resolution order.
	Config string
	// JSON is --json, the structured output for agents and scripts.
	JSON bool
	// Format is --format table|compact|md.
	Format string
	// DryRun is --dry-run on writes: nothing is sent.
	DryRun bool
	// Reason is --reason text, stored in the audit line.
	Reason string
	// Expect is --expect field=value, a precondition on writes.
	Expect []string
}

// Deps is everything a use case needs, injected by main.go and by tests.
type Deps struct {
	Reader domain.ProjectReader
	Writer domain.ProjectWriter
	Clock  domain.Clock
	// Config is loaded by the use case through config.Load with Flags;
	// nil until then.
	Config *config.Loaded
	// Dirs are the config, state and cache directories.
	Dirs config.Dirs
	// Out and Err are the command outputs.
	Out io.Writer
	Err io.Writer
	// In is the command input; nil means standard input. The MCP server
	// gives every in-process call an empty input, because its own
	// standard input carries the protocol.
	In io.Reader
	// Version is the kit version set at build time.
	Version string
	// Flags are bound to the root persistent flags.
	Flags GlobalFlags
}

// Now returns the instant of the run from the injected clock, in UTC at
// second precision: the one timestamp form every payload, plan, journal
// and audit line carries.
func (d *Deps) Now() time.Time {
	now := time.Now()
	if d.Clock != nil {
		now = d.Clock.Now()
	}
	return now.UTC().Truncate(time.Second)
}

// Command groups, one per verb tier of section 6.3.
const (
	GroupRead  = "read"
	GroupWrite = "write"
	GroupPlan  = "plan"
	GroupAuto  = "auto"
)

// NewRoot builds the "board" command tree with the global flags bound to
// deps.Flags and every tier registered.
func NewRoot(deps *Deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "board",
		Short:         "Operate a GitHub Projects v2 board safely with AI coding agents",
		Long:          "gh board lets a team and its coding agents read and move work on a GitHub Projects v2 board. Consequential changes become plans that a human applies.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
		// The output format is checked before any command runs, so a usage
		// error never waits for the board to be read.
		PersistentPreRunE: func(*cobra.Command, []string) error { return checkFormat(deps.Flags) },
	}
	root.SetOut(deps.Out)
	root.SetErr(deps.Err)
	if deps.In != nil {
		root.SetIn(deps.In)
	}
	bindGlobalFlags(root, &deps.Flags)
	registerReadCommands(root, deps)
	registerWriteCommands(root, deps)
	registerPlanCommands(root, deps)
	registerAutoCommands(root, deps)
	return root
}

func bindGlobalFlags(root *cobra.Command, flags *GlobalFlags) {
	pf := root.PersistentFlags()
	pf.StringVar(&flags.Project, "project", "", "project as owner/number, overrides board.yml")
	pf.StringVar(&flags.Config, "config", "", "path to board.yml")
	pf.BoolVar(&flags.JSON, "json", false, "structured JSON output")
	pf.StringVar(&flags.Format, "format", "table", "output format: table, compact or md")
	pf.BoolVar(&flags.DryRun, "dry-run", false, "show the write without sending it")
	pf.StringVar(&flags.Reason, "reason", "", "reason stored in the audit line")
	pf.StringArrayVar(&flags.Expect, "expect", nil, "precondition field=value for a write")
}

// checkFormat validates --format; --json needs no format.
func checkFormat(flags GlobalFlags) error {
	if flags.JSON || flags.Format == "" {
		return nil
	}
	_, err := render.ParseFormat(flags.Format)
	return err
}

// Execute runs the command tree with args and returns the command error,
// which main.go maps to an exit code with domain.CodeOf.
func Execute(ctx context.Context, args []string, deps *Deps) error {
	root := NewRoot(deps)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
