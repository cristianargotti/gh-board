package main

import (
	"path/filepath"
	"strings"
)

func isolatedEnv(parent []string, home string) []string {
	env := make([]string, 0, len(parent))
	for _, entry := range parent {
		key, _, _ := strings.Cut(entry, "=")
		if !privateVariable(strings.ToUpper(key)) {
			env = append(env, entry)
		}
	}
	paths := map[string]string{
		"HOME": home, "USERPROFILE": home,
		"APPDATA": filepath.Join(home, "appdata"), "LOCALAPPDATA": filepath.Join(home, "localappdata"),
		"XDG_CONFIG_HOME": filepath.Join(home, "config"), "XDG_DATA_HOME": filepath.Join(home, "data"),
		"XDG_STATE_HOME": filepath.Join(home, "state"), "XDG_CACHE_HOME": filepath.Join(home, "cache"),
		"GH_CONFIG_DIR": filepath.Join(home, "gh"), "GH_BOARD_HOME": filepath.Join(home, "board"),
		"CODEX_HOME": filepath.Join(home, ".codex"), "CLAUDE_CONFIG_DIR": filepath.Join(home, ".claude"),
	}
	for key, value := range paths {
		env = append(env, key+"="+value)
	}
	// gh requires authentication even for a local install; no API call uses this placeholder.
	return append(env, "GH_TOKEN=gh-board-acceptance-offline", "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1",
		"GH_NO_EXTENSION_UPDATE_NOTIFIER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1")
}

func privateVariable(key string) bool {
	switch key {
	case "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "APPDATA", "LOCALAPPDATA", "DISABLE_AUTOUPDATER":
		return true
	}
	for _, prefix := range []string{"GH_", "GITHUB_", "XDG_", "CODEX_", "CLAUDE_", "ANTHROPIC_", "OPENAI_"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
