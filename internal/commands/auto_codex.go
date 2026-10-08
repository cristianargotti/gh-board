package commands

import (
	"path/filepath"

	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoPlanCodex lists the Codex artifacts of section 8.3: the skill, the
// PreToolUse hook merged into the user hooks.json, the execpolicy rules
// file as the second layer (hook failures do not block in Codex) and, with
// project scope, the marked block in AGENTS.md.
func autoPlanCodex(scope autoScope, strict bool, roots autoRoots) ([]autoArtifact, error) {
	skill, err := autoAsset(autoAssetSkill, "")
	if err != nil {
		return nil, err
	}
	hook, err := autoHookFragment(guard.CodexHook, strict, "codex hook")
	if err != nil {
		return nil, err
	}
	rules, err := guard.CodexRules(strict)
	if err != nil {
		return nil, err
	}
	dir := autoAgentDir(roots.Home, guard.AgentCodex)
	artifacts := []autoArtifact{
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: filepath.Join(dir, autoHooksFile), Fragment: hook},
		{Kind: autoKindGuard, Type: autoTypeFile, Path: filepath.Join(dir, autoRulesDir, autoCodexRulesFile), Content: rules},
		{Kind: autoKindSkill, Type: autoTypeFile, Path: autoSkillPath(roots.Home, guard.AgentCodex), Content: skill, OwnDir: true},
	}
	if scope != autoScopeProject {
		return artifacts, nil
	}
	block, err := autoAsset(autoAssetCodexBlock, autoCodexBlockFallback)
	if err != nil {
		return nil, err
	}
	agentsPath := filepath.Join(roots.Project, autoAgentsFile)
	return append(artifacts, autoArtifact{Kind: autoKindSkill, Type: autoTypeBlock, Path: agentsPath, Content: block}), nil
}

// autoPlanArtifacts lists what an agent needs in a scope.
func autoPlanArtifacts(agent guard.Agent, scope autoScope, strict bool, roots autoRoots) ([]autoArtifact, error) {
	switch agent {
	case guard.AgentClaude:
		return autoPlanClaude(scope, strict, roots)
	case guard.AgentCursor:
		return autoPlanCursor(scope, strict, roots)
	default:
		return autoPlanCodex(scope, strict, roots)
	}
}
