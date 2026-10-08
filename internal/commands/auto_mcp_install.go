package commands

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/gh-board/internal/guard"
)

// Files that hold the MCP servers of each agent, verified against each one:
// Claude Code keeps user servers at the top level of ~/.claude.json and
// project servers in .mcp.json at the project root; Cursor reads
// mcp.json under ~/.cursor or the project .cursor; Codex reads
// [mcp_servers.<name>] tables in ~/.codex/config.toml, the shape codex
// mcp add writes.
const (
	autoClaudeUserMCP    = ".claude.json"
	autoClaudeProjectMCP = ".mcp.json"
	autoCursorMCPFile    = "mcp.json"
	autoCodexConfigFile  = "config.toml"
	autoMCPServersKey    = "mcpServers"
	autoMCPArgument      = "mcp"
	autoMCPCommandKey    = "command"
)

// autoPlanMCP lists the MCP server entry of an agent (section 8.4): a
// stdio server running this binary with the mcp argument. Codex has no
// project scope for servers, so its entry always lives in the home.
func autoPlanMCP(agent guard.Agent, scope autoScope, roots autoRoots) ([]autoArtifact, error) {
	bin, err := guard.Binary()
	if err != nil {
		return nil, err
	}
	switch agent {
	case guard.AgentClaude:
		path := filepath.Join(roots.Home, autoClaudeUserMCP)
		if scope == autoScopeProject {
			path = filepath.Join(roots.Project, autoClaudeProjectMCP)
		}
		return autoMCPJSONArtifact(path, bin)
	case guard.AgentCursor:
		path := filepath.Join(roots.Home, autoCursorDir, autoCursorMCPFile)
		if scope == autoScopeProject {
			path = filepath.Join(roots.Project, autoCursorDir, autoCursorMCPFile)
		}
		return autoMCPJSONArtifact(path, bin)
	}
	path := filepath.Join(roots.Home, autoCodexDir, autoCodexConfigFile)
	return []autoArtifact{{Kind: autoKindMCP, Type: autoTypeTOML, Path: path, Content: autoMCPTOML(bin)}}, nil
}

// autoMCPJSONArtifact is the mcpServers fragment merged into a JSON file.
func autoMCPJSONArtifact(path, bin string) ([]autoArtifact, error) {
	fragment, err := autoFragment(map[string]any{autoMCPServersKey: map[string]any{
		autoMCPName: map[string]any{mcpKeyType: "stdio", autoMCPCommandKey: bin, "args": []string{autoMCPArgument}},
	}})
	if err != nil {
		return nil, err
	}
	return []autoArtifact{{Kind: autoKindMCP, Type: autoTypeJSON, Path: path, Fragment: fragment}}, nil
}

// autoMCPTOML is the Codex table, written as a basic string so a Windows
// path survives.
func autoMCPTOML(bin string) []byte {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(bin)
	return []byte("[" + autoMCPTable + "]\n" + autoMCPCommandKey + " = \"" + escaped + "\"\nargs = [\"" + autoMCPArgument + "\"]\n")
}

// autoMCPTable is the Codex table name of the server.
const autoMCPTable = "mcp_servers." + autoMCPName

// autoApplyTOML upserts the marked block unless the table already exists
// outside it (codex mcp add wrote one): a second definition would break
// the whole file for Codex, so the file is left alone and the note says
// why.
func autoApplyTOML(a autoArtifact, before []byte) ([]byte, map[string]any, []string, error) {
	outside := autoRemoveBlockWith(before, autoTOMLMarkers)
	if autoTOMLHasTable(outside, autoMCPTable) {
		return before, nil, []string{"[" + autoMCPTable + "] already defined outside the gh-board block, left as is"}, nil
	}
	return autoUpsertBlockWith(before, a.Content, autoTOMLMarkers), nil, nil, nil
}

// autoTOMLHasTable reports whether a table header line names the table.
func autoTOMLHasTable(text []byte, table string) bool {
	scanner := bufio.NewScanner(bytes.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.TrimSpace(strings.Trim(line, "[]")) == table {
			return true
		}
	}
	return false
}

// autoTOMLCommand reads the command of the server table inside the
// kit's block: the value of the command key as a basic string.
func autoTOMLCommand(text []byte) (string, bool) {
	start, end, ok := autoFindBlockWith(text, autoTOMLMarkers)
	if !ok {
		return "", false
	}
	scanner := bufio.NewScanner(bytes.NewReader(text[start:end]))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if !found || strings.TrimSpace(key) != autoMCPCommandKey {
			continue
		}
		return autoTOMLUnquote(strings.TrimSpace(value))
	}
	return "", false
}

// autoTOMLUnquote reads a basic string with the two escapes the kit
// writes.
func autoTOMLUnquote(value string) (string, bool) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(value[1 : len(value)-1]), true
}
