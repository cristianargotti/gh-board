package mcp

import (
	"context"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Tiers of section 6.3 a tool belongs to. The tier decides the
// annotations the client sees and the sentence appended to the
// description; apply has no tier here because it is never a tool.
const (
	TierRead  = "read"
	TierWrite = "write"
	TierPlan  = "plan"
)

// Tool is one exposed verb of the kit with the JSON Schema of its
// arguments, derived by the caller from the command tree so that the MCP
// catalog and the command line help are one catalog.
type Tool struct {
	// Name is the tool name, the verb with underscores (digest_post).
	Name string
	// Title is the human form, the command path.
	Title string
	// Description is the verb's own help text.
	Description string
	// Tier is TierRead, TierWrite or TierPlan.
	Tier string
	// InputSchema is a JSON Schema object describing the arguments.
	InputSchema map[string]any
}

// Outcome is what running a tool's command produced: the exit code of
// section 7, the --json document on standard output, what the command
// wrote on standard error and the error message when the code is not 0.
type Outcome struct {
	Code    domain.ExitCode
	Output  []byte
	Stderr  string
	Message string
}

// Runner executes a tool with validated arguments. The caller supplies
// it, so this package never imports the commands it serves.
type Runner func(ctx context.Context, tool Tool, args map[string]any) Outcome

// Server is one MCP server over a pair of streams. It serves one request
// at a time, in order, with no goroutines.
type Server struct {
	// Name and Version fill serverInfo.
	Name    string
	Version string
	// Instructions tell the client's model how to use the tools.
	Instructions string
	// Tools is the catalog, in the order tools/list returns it.
	Tools []Tool
	// Run executes a tool call.
	Run Runner

	// initialized is set by the legacy handshake; modern requests carry
	// their version on every message and need no handshake.
	initialized bool
}

// Protocol versions. Modern clients (2026-07-28) declare the version on
// every request and probe with server/discover; legacy clients open with
// an initialize handshake and negotiate one of the legacy versions.
const (
	modernVersion = "2026-07-28"
	latestLegacy  = "2025-11-25"
)

// legacyVersions are the handshake revisions this server speaks, newest
// first. The tool subset used here (tools/list, tools/call with text and
// structured content) is the same in every one of them.
var legacyVersions = []string{latestLegacy, "2025-06-18", "2025-03-26", "2024-11-05"}

// tool finds a tool by name.
func (s *Server) tool(name string) (Tool, bool) {
	for _, t := range s.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// serverInfo is the implementation block of initialize and discover.
func (s *Server) serverInfo() map[string]any {
	return map[string]any{"name": s.Name, "version": s.Version}
}

// capabilities declares tools only; the catalog never changes while the
// process runs, so no list change notification is ever sent.
func capabilities() map[string]any {
	return map[string]any{"tools": map[string]any{"listChanged": false}}
}
