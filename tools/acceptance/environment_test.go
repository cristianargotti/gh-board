package main

import (
	"path/filepath"
	"strings"
	"testing"
)

var privateKeys = []string{
	"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "APPDATA", "LocalAppData",
	"GH_TOKEN", "GH_BOARD_HOME", "GH_BOARD_CONFIG", "GITHUB_TOKEN", "XDG_DATA_HOME",
	"CODEX_HOME", "CLAUDE_CONFIG_DIR", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "DISABLE_AUTOUPDATER",
}

func TestIsolatedEnv(t *testing.T) {
	parent := []string{"PATH=/tools", "SystemRoot=C:\\Windows", "GOCACHE=/build-cache"}
	for _, key := range privateKeys {
		parent = append(parent, key+"=original-private-value")
	}
	home := filepath.Join(t.TempDir(), "home with spaces")
	env := isolatedEnv(parent, home)
	values := make(map[string]string)
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if value == "original-private-value" {
			t.Fatalf("inherited %s", key)
		}
		if _, found := values[key]; found {
			t.Fatalf("duplicate %s", key)
		}
		values[key] = value
	}
	if values["HOME"] != home || values["USERPROFILE"] != home || values["PATH"] != "/tools" {
		t.Fatal(values)
	}
	if values["CODEX_HOME"] != filepath.Join(home, ".codex") || values["GH_TOKEN"] != "gh-board-acceptance-offline" {
		t.Fatal(values)
	}
}
