package commands

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/cristianargotti/gh-board/internal/guard"
)

// autoMarketplaceName names both the Claude Code marketplace the product
// repository is (section 8.2) and the plugin inside it.
const autoMarketplaceName = "gh-board"

// autoMarketplaceSpec is the plugin pointer written into the settings.
type autoMarketplaceSpec struct {
	// Name is the key of extraKnownMarketplaces.
	Name string
	// Plugin is the plugin name inside the marketplace.
	Plugin string
	// Repo is the GitHub repository, owner/name.
	Repo string
}

// autoPlanClaude lists the Claude Code artifacts, all merged into one
// settings file (user or project scope): the deny rules in
// permissions.deny, which the engine evaluates before hooks and plugins,
// the PreToolUse hook that protects until the plugin is enabled, and the
// plugin pointer, which is the marketplace entry plus the enabled plugin
// (sections 6.5 and 8.2). The plugin carries the skill and the mod.
func autoPlanClaude(scope autoScope, strict bool, roots autoRoots) ([]autoArtifact, error) {
	deny, err := autoClaudeDenyFragment(strict)
	if err != nil {
		return nil, err
	}
	hook, err := autoHookFragment(guard.ClaudeHook, strict, "claude hook")
	if err != nil {
		return nil, err
	}
	spec, err := autoMarketplace(autoModulePath())
	if err != nil {
		return nil, err
	}
	pointer, err := autoFragment(autoPluginPointer(spec))
	if err != nil {
		return nil, err
	}
	settings := filepath.Join(roots.Home, autoClaudeDir, autoClaudeSettings)
	if scope == autoScopeProject {
		settings = filepath.Join(roots.Project, autoClaudeDir, autoClaudeSettings)
	}
	return []autoArtifact{
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: settings, Fragment: deny},
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: settings, Fragment: hook},
		{Kind: autoKindSkill, Type: autoTypeJSON, Path: settings, Fragment: pointer},
	}, nil
}

// autoClaudeDenyFragment is the permissions.deny fragment of the guard.
func autoClaudeDenyFragment(strict bool) (map[string]any, error) {
	rules, err := guard.ClaudeDenyRules(strict)
	if err != nil {
		return nil, err
	}
	deny := make([]string, 0, len(rules))
	for _, r := range rules {
		deny = append(deny, autoBashRule(r))
	}
	return autoFragment(map[string]any{"permissions": map[string]any{"deny": deny}})
}

// autoBashRule wraps a guard rule in the Bash( ) form of permissions.deny,
// unless the guard already did.
func autoBashRule(rule string) string {
	if strings.HasPrefix(rule, "Bash(") && strings.HasSuffix(rule, ")") {
		return rule
	}
	return "Bash(" + rule + ")"
}

// autoPluginPointer is the settings fragment that makes Claude Code know
// the marketplace and enable the plugin.
func autoPluginPointer(m autoMarketplaceSpec) map[string]any {
	return map[string]any{
		"extraKnownMarketplaces": map[string]any{
			m.Name: map[string]any{"source": map[string]any{"source": "github", "repo": m.Repo}},
		},
		"enabledPlugins": map[string]any{m.Plugin + "@" + m.Name: true},
	}
}

// autoMarketplace derives the marketplace repository from the module path
// the kit was built from, so that a fork or a transfer of the repository
// moves the pointer with the module path instead of a hardcoded login.
func autoMarketplace(modulePath string) (autoMarketplaceSpec, error) {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 || parts[0] != "github.com" || parts[1] == "" || parts[2] == "" {
		return autoMarketplaceSpec{}, fmt.Errorf("module path %q is not a GitHub repository, the Claude Code marketplace cannot be named", modulePath)
	}
	return autoMarketplaceSpec{Name: autoMarketplaceName, Plugin: autoMarketplaceName, Repo: parts[1] + "/" + parts[2]}, nil
}

// autoModulePath is the main module of the running binary.
func autoModulePath() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Path
}
