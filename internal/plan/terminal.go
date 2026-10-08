package plan

import (
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/term"
)

// IsTerminal checks the input file with go-gh so non-terminal character
// devices cannot authorize apply. Commands pass os.Stdin. On Windows a
// Git Bash or mintty pty is a named pipe the console API does not see,
// so the pipe name is checked the way gh itself does.
func IsTerminal(f *os.File) bool {
	return isTerminal(f, term.IsTerminal) || isCygwinTerminal(f)
}

func isTerminal(f *os.File, detect func(*os.File) bool) bool {
	return f != nil && detect(f)
}

// isCygwinTerminal reports whether the file is an MSYS or Cygwin pty,
// through the pipe name the platform exposes.
func isCygwinTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	name, ok := pipeName(f)
	return ok && cygwinPtyName(name)
}

// cygwinPtyName reports whether a Windows pipe name is an MSYS or Cygwin
// pseudo terminal: \msys-<id>-pty<n>-{from,to}-master, also under the
// \Device\NamedPipe prefix.
func cygwinPtyName(name string) bool {
	parts := strings.Split(name, "-")
	if len(parts) < 5 {
		return false
	}
	switch parts[0] {
	case `\msys`, `\cygwin`, `\Device\NamedPipe\msys`, `\Device\NamedPipe\cygwin`:
	default:
		return false
	}
	return parts[1] != "" && strings.HasPrefix(parts[2], "pty") &&
		(parts[3] == "from" || parts[3] == "to") && parts[4] == "master"
}
