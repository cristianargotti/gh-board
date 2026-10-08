package commands

import (
	"path/filepath"

	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoPlanCursor lists the Cursor artifacts of section 8.3: the skill under
// the user skills directory, the beforeShellExecution hook merged into the
// user hooks.json and, with project scope, the rule file that points the
// repository at the skill.
func autoPlanCursor(scope autoScope, strict bool, roots autoRoots) ([]autoArtifact, error) {
	skill, err := autoAsset(autoAssetSkill, "")
	if err != nil {
		return nil, err
	}
	hook, err := autoHookFragment(guard.CursorHook, strict, "cursor hook")
	if err != nil {
		return nil, err
	}
	dir := autoAgentDir(roots.Home, guard.AgentCursor)
	artifacts := []autoArtifact{
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: filepath.Join(dir, autoHooksFile), Fragment: hook},
		{Kind: autoKindSkill, Type: autoTypeFile, Path: autoSkillPath(roots.Home, guard.AgentCursor), Content: skill, OwnDir: true},
	}
	if scope != autoScopeProject {
		return artifacts, nil
	}
	rule, err := autoAsset(autoAssetCursorRule, autoCursorRuleFallback)
	if err != nil {
		return nil, err
	}
	rulePath := filepath.Join(roots.Project, autoCursorDir, autoRulesDir, autoCursorRuleFile)
	return append(artifacts, autoArtifact{Kind: autoKindSkill, Type: autoTypeFile, Path: rulePath, Content: rule}), nil
}

// autoHookFragment generates a hook fragment through the guard and decodes
// it for the JSON merge.
func autoHookFragment(gen func(bool) ([]byte, error), strict bool, name string) (map[string]any, error) {
	data, err := gen(strict)
	if err != nil {
		return nil, err
	}
	return autoJSONObject(data, name)
}
