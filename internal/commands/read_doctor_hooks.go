package commands

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cristianargotti/gh-board/internal/guard"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readDoctorHook is one installed hook command with the binary it runs.
type readDoctorHook struct {
	Agent  string `json:"agent"`
	Scope  string `json:"scope"`
	Path   string `json:"path"`
	Binary string `json:"binary,omitempty"`
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// readDoctorHooksCheck compares the binary every installed hook names with
// the running one (section 6.5): the hooks judge commands with the binary
// they name, so a hook left behind by another installation protects with
// another kit, or with none.
func readDoctorHooksCheck(deps *Deps, rep *readDoctorReport) {
	hooks, err := autoInstalledHookCommands(deps.Dirs)
	if err != nil {
		rep.HooksNote = "agent hooks unreadable: " + err.Error()
		rep.problem(rep.HooksNote)
		return
	}
	if len(hooks) == 0 {
		rep.HooksNote = "no agent hooks recorded: install the guards with gh board agent install"
		return
	}
	running, err := readRunningBinary()
	if err != nil {
		rep.HooksNote = readNoteBinaryPath + err.Error()
		rep.problem(rep.HooksNote)
		return
	}
	for _, h := range hooks {
		row := readDoctorHook{Agent: string(h.Agent), Scope: string(h.Scope), Path: h.Path}
		switch bin, ok := guard.HookBinary(h.Command); {
		case !ok:
			row.Reason = "hook command not written by this kit: " + render.Sanitize(h.Command, 80)
		case readSamePath(bin, running):
			row.Binary, row.OK = bin, true
		default:
			row.Binary = bin
			row.Reason = fmt.Sprintf("names %s, this binary is %s", bin, running)
		}
		if !row.OK {
			rep.problem(fmt.Sprintf("hook for %s in %s: %s", h.Agent, h.Path, row.Reason))
		}
		rep.Hooks = append(rep.Hooks, row)
	}
}

// readRunningBinary resolves the running binary the way the installers
// name it: absolute, symlinks followed when the file exists.
func readRunningBinary() (string, error) {
	path, err := readExecutable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Abs(path)
}

// readSamePath compares two binary paths, ignoring case where the file
// system does.
func readSamePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
