package main

import (
	"runtime/debug"
	"strings"
)

// githubHost is the prefix of a module path hosted on GitHub.
const githubHost = "github.com/"

// readBuildInfo is replaced by tests to exercise a binary without build
// information.
var readBuildInfo = debug.ReadBuildInfo

// releaseRepository names the owner/name that publishes the kit releases,
// derived from the main module path so that a fork or a transfer moves it
// without a code change; empty when the binary carries no build info or
// the module is not hosted on GitHub.
func releaseRepository() string {
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return ""
	}
	return repositoryOf(info.Main.Path)
}

// repositoryOf keeps the owner and name of a GitHub module path.
func repositoryOf(modulePath string) string {
	rest, ok := strings.CutPrefix(modulePath, githubHost)
	if !ok {
		return ""
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1]
}
