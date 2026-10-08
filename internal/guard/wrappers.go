package guard

import "strings"

// wrapper describes a program that runs its arguments as the actual
// command. values lists the options that take a separate value, positional
// counts the operands before the command (the duration of timeout) and
// refuse recognizes the forms that are not wrappers (command -v).
type wrapper struct {
	values     map[string]bool
	positional int
	refuse     func(args []string) bool
}

func set(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// wrappers holds the set Claude Code strips before matching its rules
// (timeout, time, nice, nohup, stdbuf, command, builtin, noglob and bare
// xargs, verified against its documentation) plus the exec-style wrappers that run their
// argument the same way and cost nothing to see through.
var wrappers = map[string]wrapper{
	"timeout":   {values: set("-k", "-s", "--kill-after", "--signal"), positional: 1},
	"time":      {values: set("-f", "-o", "--format", "--output")},
	"nice":      {values: set("-n", "--adjustment")},
	"nohup":     {},
	"stdbuf":    {values: set("-i", "-o", "-e", "--input", "--output", "--error")},
	"command":   {refuse: queryForm},
	"builtin":   {},
	"noglob":    {},
	"nocorrect": {},
	"xargs":     {refuse: hasOptions},
	"exec":      {values: set("-a")},
	"sudo": {values: set(
		"-u", "-g", "-C", "-D", "-h", "-p", "-r", "-t", "-T", "-U",
		"--user", "--group", "--chdir", "--close-from", "--host", "--prompt",
		"--role", "--type", "--other-user", "--command-timeout",
	)},
	"doas":   {values: set("-u", "-C")},
	"setsid": {},
	"ionice": {values: set("-c", "-n", "-p", "-P", "-u", "--class", "--classdata", "--pid", "--pgid", "--uid")},
}

// shellPrograms run a script given with -c; the argv form of the Codex
// hook and the Codex rules file both need to recognize them.
var shellPrograms = set("sh", "bash", "zsh", "dash", "ksh", "fish", "pwsh", "powershell", "cmd")

// queryForm is "command -v name" or "command -V name", which look a
// command up instead of running it.
func queryForm(args []string) bool {
	return len(args) > 0 && (strings.HasPrefix(args[0], "-v") || strings.HasPrefix(args[0], "-V"))
}

// hasOptions keeps xargs with flags as an xargs command, the way Claude
// Code does: only bare xargs is stripped.
func hasOptions(args []string) bool {
	return len(args) > 0 && strings.HasPrefix(args[0], "-")
}

// strip returns the command that follows the wrapper and its options, or
// false when the words are not a wrapper form.
func (w wrapper) strip(args []string) ([]string, bool) {
	if w.refuse != nil && w.refuse(args) {
		return nil, false
	}
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		opt := args[i]
		i++
		if opt == "--" {
			break
		}
		if w.values[opt] {
			i++
		}
	}
	i += w.positional
	if i > len(args) {
		return nil, false
	}
	return args[i:], true
}
