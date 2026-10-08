package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoScope is the --scope value of the installers: user writes the agent
// directories under the home; project adds the files of the repository in
// the working directory (section 8.3).
type autoScope string

// Installer scopes.
const (
	autoScopeUser    autoScope = "user"
	autoScopeProject autoScope = "project"
)

// autoParseScope validates a --scope value.
func autoParseScope(s string) (autoScope, error) {
	switch scope := autoScope(strings.ToLower(strings.TrimSpace(s))); scope {
	case autoScopeUser, autoScopeProject:
		return scope, nil
	}
	return "", fmt.Errorf("scope %q: expected user or project: %w", s, domain.ErrUsage)
}

// autoRoots are the two directories the installers write under.
type autoRoots struct {
	// Home is the user's home directory, where the agents keep settings.
	Home string
	// Project is the working directory; empty unless the scope is project.
	Project string
}

// autoResolveRoots resolves the roots of a scope.
func autoResolveRoots(scope autoScope) (autoRoots, error) {
	home, err := autoUserHome()
	if err != nil {
		return autoRoots{}, fmt.Errorf("resolve the home directory: %w", err)
	}
	roots := autoRoots{Home: home}
	if scope != autoScopeProject {
		return roots, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return autoRoots{}, fmt.Errorf("resolve the working directory: %w", err)
	}
	roots.Project = wd
	return roots, nil
}

// autoUserHome is where the three agents keep their settings; config is
// the one place that reads directories, and tests point HOME at a
// temporary directory.
func autoUserHome() (string, error) {
	return config.UserHomeDir()
}

// Agent directory and file names, as each agent documents them.
const (
	autoClaudeDir      = ".claude"
	autoClaudeSettings = "settings.json"
	autoCursorDir      = ".cursor"
	autoCodexDir       = ".codex"
	autoHooksFile      = "hooks.json"
	autoSkillsDir      = "skills"
	autoRulesDir       = "rules"
	autoSkillName      = "gh-board"
	autoSkillFile      = "SKILL.md"
	autoCursorRuleFile = "gh-board.mdc"
	autoCodexRulesFile = "gh-board.rules"
	autoAgentsFile     = "AGENTS.md"
)

// autoAgentDir returns the user directory of an agent.
func autoAgentDir(home string, agent guard.Agent) string {
	switch agent {
	case guard.AgentClaude:
		return filepath.Join(home, autoClaudeDir)
	case guard.AgentCursor:
		return filepath.Join(home, autoCursorDir)
	case guard.AgentCodex:
		return filepath.Join(home, autoCodexDir)
	}
	return ""
}

// autoSkillPath is where an agent reads the installed skill.
func autoSkillPath(home string, agent guard.Agent) string {
	return filepath.Join(autoAgentDir(home, agent), autoSkillsDir, autoSkillName, autoSkillFile)
}

// autoSelectAgents expands an --agent value into the agents to set up.
func autoSelectAgents(s string) ([]guard.Agent, error) {
	if strings.EqualFold(strings.TrimSpace(s), "all") || strings.TrimSpace(s) == "" {
		return append([]guard.Agent(nil), guard.Agents...), nil
	}
	agent, err := guard.ParseAgent(s)
	if err != nil {
		return nil, err
	}
	return []guard.Agent{agent}, nil
}
