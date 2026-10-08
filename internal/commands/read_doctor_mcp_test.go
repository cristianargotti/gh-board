package commands

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// readMCPReceipt records MCP entries of the kinds and paths given.
func readMCPReceipt(t *testing.T, deps *Deps, agent string, entries map[string]autoArtifactType) {
	t.Helper()
	receipt := map[string]any{"agent": agent, "scope": "user", "artifacts": []map[string]any{}}
	for path, kind := range entries {
		receipt["artifacts"] = append(receipt["artifacts"].([]map[string]any), map[string]any{
			"kind": "mcp", "type": string(kind), "path": path, "sha256": strings.Repeat("0", 64),
		})
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	readWriteFile(t, filepath.Join(deps.Dirs.Config, "agents", agent+"-user.json"), string(data))
}

// readMCPJSON encodes a Claude Code MCP entry for the binary; json.Marshal
// keeps Windows paths valid inside the JSON string.
func readMCPJSON(t *testing.T, binary string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"gh-board": map[string]any{"command": binary, "args": []string{"mcp"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReadDoctorMCPEntries(t *testing.T) {
	readPinEnvironment(t)
	deps, out := readTestDeps(t, readNewFakeReader())
	home := t.TempDir()
	binary := filepath.Join(home, "gh-board")
	readStubExecutable(t, binary)
	ok := filepath.Join(home, "ok.json")
	other := filepath.Join(home, "other.json")
	empty := filepath.Join(home, "empty.json")
	toml := filepath.Join(home, "config.toml")
	bare := filepath.Join(home, "bare.toml")
	readWriteFile(t, ok, readMCPJSON(t, binary))
	readWriteFile(t, other, `{"mcpServers":{"gh-board":{"command":"/elsewhere/gh-board","args":["mcp"]}}}`)
	readWriteFile(t, empty, `{"mcpServers":{}}`)
	readWriteFile(t, toml, string(autoUpsertBlockWith(nil, autoMCPTOML(binary), autoTOMLMarkers)))
	readWriteFile(t, bare, "model = \"o3\"\n")
	readMCPReceipt(t, deps, "claude", map[string]autoArtifactType{ok: autoTypeJSON, other: autoTypeJSON, empty: autoTypeJSON, filepath.Join(home, "none.json"): autoTypeJSON})
	readMCPReceipt(t, deps, "codex", map[string]autoArtifactType{toml: autoTypeTOML, bare: autoTypeTOML})
	if err := readRun(t, deps, "doctor"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"MCP servers", "names /elsewhere/gh-board, this binary is " + binary, "entry missing", "missing",
		"MCP server for claude in " + other,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Count(out.String(), "  ok") < 2 {
		t.Fatalf("the JSON and TOML entries naming this binary must pass:\n%s", out.String())
	}
}

func TestReadDoctorMCPNotes(t *testing.T) {
	readPinEnvironment(t)
	deps, out := readTestDeps(t, readNewFakeReader())
	if err := readRun(t, deps, "doctor"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no MCP servers recorded: register them with gh board agent install --mcp") {
		t.Fatalf("output:\n%s", out.String())
	}
	readMCPReceipt(t, deps, "cursor", map[string]autoArtifactType{filepath.Join(t.TempDir(), "x.json"): autoTypeJSON})
	readStubExecutableError(t, errors.New("no exe"))
	out.Reset()
	if err := readRun(t, deps, "doctor"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "MCP servers\n  binary path unavailable: no exe") {
		t.Fatalf("output:\n%s", out.String())
	}
	readWriteFile(t, filepath.Join(deps.Dirs.Config, "agents", "cursor-user.json"), "{")
	out.Reset()
	if err := readRun(t, deps, "doctor"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "MCP server entries unreadable") {
		t.Fatalf("output:\n%s", out.String())
	}
}
