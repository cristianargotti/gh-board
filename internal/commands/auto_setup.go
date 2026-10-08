package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/guard"
	"github.com/cristianargotti/gh-board/internal/render"
)

// autoSetupOptions are the flags shared by agent install|uninstall and
// guard install|uninstall.
type autoSetupOptions struct {
	Agent  string
	Strict bool
	Scope  string
	MCP    bool
}

// autoSetupSpec describes one of the four setup commands.
type autoSetupSpec struct {
	Use     string
	Short   string
	Long    string
	Kinds   []autoKind
	Install bool
	// MCP offers the --mcp flag: agent install adds the server entry with
	// it, agent uninstall removes only that entry with it.
	MCP bool
}

// autoFileReport is one file of the JSON report.
type autoFileReport struct {
	Path   string   `json:"path"`
	Kind   autoKind `json:"kind"`
	Action string   `json:"action"`
	Notes  []string `json:"notes,omitempty"`
}

// autoAgentReport is one agent of the JSON report.
type autoAgentReport struct {
	Agent guard.Agent      `json:"agent"`
	Files []autoFileReport `json:"files"`
}

// autoSetupReport is the --json form of a setup run.
type autoSetupReport struct {
	Command string            `json:"command"`
	Scope   autoScope         `json:"scope"`
	Strict  bool              `json:"strict"`
	MCP     bool              `json:"mcp"`
	DryRun  bool              `json:"dry_run"`
	Agents  []autoAgentReport `json:"agents"`
}

// autoSetupCommand builds an install or uninstall command.
func autoSetupCommand(deps *Deps, spec autoSetupSpec) *cobra.Command {
	var opts autoSetupOptions
	cmd := &cobra.Command{
		Use:     spec.Use,
		Short:   spec.Short,
		Long:    spec.Long,
		GroupID: GroupAuto,
		Args:    cobra.NoArgs,
		RunE:    func(*cobra.Command, []string) error { return autoRunSetup(deps, spec, opts) },
	}
	f := cmd.Flags()
	f.StringVar(&opts.Agent, "agent", "all", "agent to set up: claude, cursor, codex or all")
	f.StringVar(&opts.Scope, "scope", string(autoScopeUser), "user writes the agent directories under the home; project also writes the repository files in the working directory")
	f.BoolVar(&opts.Strict, "strict", false, "install strict guards for wrappers, absolute gh paths and file bodies; uninstall removes all recorded guards")
	if spec.MCP {
		f.BoolVar(&opts.MCP, "mcp", false, "install: also register the MCP server (gh board mcp) in the agent; uninstall: remove only that entry")
	}
	return cmd
}

// autoSetupKinds selects the artifact kinds of a run: uninstall --mcp
// touches the server entry alone.
func autoSetupKinds(spec autoSetupSpec, mcp bool) []autoKind {
	if mcp && !spec.Install {
		return []autoKind{autoKindMCP}
	}
	return spec.Kinds
}

// autoRunSetup runs a setup command over the selected agents and prints
// the diffs before the writes and a report after them.
func autoRunSetup(deps *Deps, spec autoSetupSpec, opts autoSetupOptions) error {
	agents, err := autoSelectAgents(opts.Agent)
	if err != nil {
		return err
	}
	scope, err := autoParseScope(opts.Scope)
	if err != nil {
		return err
	}
	roots, err := autoResolveRoots(scope)
	if err != nil {
		return err
	}
	ws := autoNewWorkspace(autoDiffWriter(deps), deps.Flags.DryRun)
	report := autoSetupReport{Command: spec.Use, Scope: scope, Strict: opts.Strict, MCP: opts.MCP, DryRun: deps.Flags.DryRun}
	doc := render.NewDocument(autoSetupTitle(spec, deps.Flags.DryRun))
	spec.Kinds = autoSetupKinds(spec, opts.MCP)
	for _, agent := range agents {
		run := autoSetupRun{deps: deps, ws: ws, spec: spec, agent: agent, scope: scope, strict: opts.Strict, mcp: opts.MCP, roots: roots}
		results, err := run.execute()
		if err != nil {
			return fmt.Errorf("%s: %w", agent, err)
		}
		report.Agents = append(report.Agents, autoAgentReport{Agent: agent, Files: autoFileReports(results)})
		autoAddSetupSection(doc, agent, results)
	}
	doc.Data = report
	return autoRender(deps, doc)
}

// autoSetupTitle names the report after the command and the mode.
func autoSetupTitle(spec autoSetupSpec, dryRun bool) string {
	verb := "Installed"
	if !spec.Install {
		verb = "Removed"
	}
	layer := "agent kit"
	if len(spec.Kinds) == 1 && spec.Kinds[0] == autoKindGuard {
		layer = "guard layer"
	}
	if dryRun {
		return fmt.Sprintf("%s %s (dry run, nothing written)", verb, layer)
	}
	return fmt.Sprintf("%s %s", verb, layer)
}

func autoFileReports(results []autoResult) []autoFileReport {
	out := make([]autoFileReport, 0, len(results))
	for _, r := range results {
		out = append(out, autoFileReport{Path: r.Path, Kind: r.Entry.Kind, Action: r.Action, Notes: r.Notes})
	}
	return out
}

// autoAddSetupSection adds one agent's files to the document.
func autoAddSetupSection(doc *render.Document, agent guard.Agent, results []autoResult) {
	section := doc.AddSection(string(agent))
	if len(results) == 0 {
		section.AddNote("nothing to do")
		return
	}
	table := section.SetTable("File", "Action", "Notes")
	for _, r := range results {
		table.AddRow(r.Path, r.Action, strings.Join(r.Notes, "; "))
	}
}
