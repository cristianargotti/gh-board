// Package guard holds the destructive-command patterns of section 6.5, the
// evaluators for the Claude Code, Cursor and Codex hook formats and the
// generators of the rules each agent installs. The patterns are one
// embedded source, patterns.json, and every other format derives from it.
package guard

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Agent is one of the three supported coding agents.
type Agent string

// Supported agents.
const (
	AgentClaude Agent = "claude"
	AgentCursor Agent = "cursor"
	AgentCodex  Agent = "codex"
)

// Agents lists the supported agents in installer order.
var Agents = []Agent{AgentClaude, AgentCursor, AgentCodex}

// ParseAgent validates an --agent value.
func ParseAgent(s string) (Agent, error) {
	for _, a := range Agents {
		if string(a) == strings.ToLower(strings.TrimSpace(s)) {
			return a, nil
		}
	}
	return "", fmt.Errorf("agent %q: expected claude, cursor or codex: %w", s, domain.ErrUsage)
}

// Answer is what guard check writes for the agent. Body goes to stdout in
// the agent's documented shape, Message is the English denial reason for
// stderr, which Claude Code and Codex read when the exit code is 2, and
// ExitCode is the process status: 2 denies in Claude Code and Codex, while
// Cursor reads the JSON permission and gets 0.
type Answer struct {
	Body     []byte
	Message  string
	ExitCode int
}

// Artifact is an installed file with the SHA-256 the installer recorded.
type Artifact struct {
	Path   string
	SHA256 string
}

// VerifyResult is the state of one installed artifact for doctor.
type VerifyResult struct {
	Path   string
	OK     bool
	Reason string
}
