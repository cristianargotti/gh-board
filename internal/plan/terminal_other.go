//go:build !windows

package plan

import "os"

// pipeName is a Windows concern: elsewhere a pty is a terminal to the
// console API already, so no name is read.
func pipeName(*os.File) (string, bool) { return "", false }
