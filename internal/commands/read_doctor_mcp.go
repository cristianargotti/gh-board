package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// readNoteBinaryPath opens every note about an unresolvable binary.
const readNoteBinaryPath = "binary path unavailable: "

// autoMCPEntry is one MCP server entry an installer recorded.
type autoMCPEntry struct {
	Agent guard.Agent
	Scope autoScope
	Path  string
	Type  autoArtifactType
}

// autoInstalledMCP lists the MCP server entries of every receipt.
func autoInstalledMCP(dirs config.Dirs) ([]autoMCPEntry, error) {
	paths, err := filepath.Glob(filepath.Join(dirs.Config, autoReceiptsDir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []autoMCPEntry
	for _, p := range paths {
		r, found, err := autoReadReceipt(p)
		if err != nil || !found {
			return nil, err
		}
		for _, e := range r.Entries {
			if e.Kind == autoKindMCP {
				out = append(out, autoMCPEntry{Agent: r.Agent, Scope: r.Scope, Path: e.Path, Type: e.Type})
			}
		}
	}
	return out, nil
}

// autoMCPBinaryIn reads the binary the gh-board server entry of an agent
// file names: the command of the TOML table inside the kit's block, or
// the command of mcpServers.gh-board in a JSON file.
func autoMCPBinaryIn(path string, kind autoArtifactType) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // agent file the installer recorded
	if errors.Is(err, fs.ErrNotExist) {
		return "", errors.New("missing")
	}
	if err != nil {
		return "", fmt.Errorf("unreadable: %w", err)
	}
	if kind == autoTypeTOML {
		bin, ok := autoTOMLCommand(data)
		if !ok {
			return "", errors.New("entry missing")
		}
		return bin, nil
	}
	obj, err := autoJSONObject(data, path)
	if err != nil {
		return "", err
	}
	servers, _ := obj[autoMCPServersKey].(map[string]any)
	entry, _ := servers[autoMCPName].(map[string]any)
	bin, ok := entry[autoMCPCommandKey].(string)
	if !ok || bin == "" {
		return "", errors.New("entry missing")
	}
	return bin, nil
}

// readDoctorMCPCheck compares the binary every recorded MCP server entry
// names with the running one, the way the hooks check does: an entry
// left by another installation starts another kit, or nothing.
func readDoctorMCPCheck(deps *Deps, rep *readDoctorReport) {
	entries, err := autoInstalledMCP(deps.Dirs)
	if err != nil {
		rep.MCPNote = "MCP server entries unreadable: " + err.Error()
		rep.problem(rep.MCPNote)
		return
	}
	if len(entries) == 0 {
		rep.MCPNote = "no MCP servers recorded: register them with gh board agent install --mcp"
		return
	}
	running, err := readRunningBinary()
	if err != nil {
		rep.MCPNote = readNoteBinaryPath + err.Error()
		rep.problem(rep.MCPNote)
		return
	}
	for _, e := range entries {
		row := readDoctorHook{Agent: string(e.Agent), Scope: string(e.Scope), Path: e.Path}
		switch bin, err := autoMCPBinaryIn(e.Path, e.Type); {
		case err != nil:
			row.Reason = err.Error()
		case readSamePath(bin, running):
			row.Binary, row.OK = bin, true
		default:
			row.Binary = bin
			row.Reason = fmt.Sprintf("names %s, this binary is %s", bin, running)
		}
		if !row.OK {
			rep.problem(fmt.Sprintf("MCP server for %s in %s: %s", e.Agent, e.Path, row.Reason))
		}
		rep.MCP = append(rep.MCP, row)
	}
}
