package commands

import (
	"path/filepath"
	"sort"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoHookCommand is one hook command an installer merged into an agent
// file, as the receipt recorded it.
type autoHookCommand struct {
	Agent   guard.Agent
	Scope   autoScope
	Path    string
	Command string
}

// autoInstalledHookCommands lists the hook commands the receipts hold, so
// doctor can check which binary every installed hook runs. The deltas of
// the guard fragments carry them under a "command" key, whatever the
// agent's hook schema around it.
func autoInstalledHookCommands(dirs config.Dirs) ([]autoHookCommand, error) {
	paths, err := filepath.Glob(filepath.Join(dirs.Config, autoReceiptsDir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []autoHookCommand
	for _, p := range paths {
		r, found, err := autoReadReceipt(p)
		if err != nil || !found {
			return nil, err
		}
		for _, e := range r.Entries {
			if e.Kind != autoKindGuard || e.Type != autoTypeJSON {
				continue
			}
			var commands []string
			autoCommandStrings(e.Delta, &commands)
			sort.Strings(commands)
			for _, c := range commands {
				out = append(out, autoHookCommand{Agent: r.Agent, Scope: r.Scope, Path: e.Path, Command: c})
			}
		}
	}
	return out, nil
}

// autoCommandStrings collects the string values of every "command" key
// in a decoded JSON fragment, at any depth.
func autoCommandStrings(v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for key, value := range t {
			if s, ok := value.(string); ok && key == "command" {
				*out = append(*out, s)
				continue
			}
			autoCommandStrings(value, out)
		}
	case []any:
		for _, e := range t {
			autoCommandStrings(e, out)
		}
	}
}
