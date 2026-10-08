package config_test

import (
	"os"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
)

var environmentCases = []struct {
	name   string
	values map[string]string
	want   config.Environment
	shadow string
}{
	{"nothing set", nil, config.Environment{}, ""},
	{
		"paths only",
		map[string]string{config.EnvConfig: "/tmp/board.yml", config.EnvHome: "/tmp/home"},
		config.Environment{ConfigPath: "/tmp/board.yml", Home: "/tmp/home"},
		"",
	},
	{"GITHUB_TOKEN alone", map[string]string{config.EnvGitHubToken: "x"}, config.Environment{GitHubToken: true}, config.EnvGitHubToken},
	{"GH_TOKEN alone", map[string]string{config.EnvGHToken: "x"}, config.Environment{GHToken: true}, config.EnvGHToken},
	{"GH_TOKEN wins", map[string]string{config.EnvGHToken: "x", config.EnvGitHubToken: "y"}, config.Environment{GHToken: true, GitHubToken: true}, config.EnvGHToken},
	{"empty values do not count", map[string]string{config.EnvGHToken: "", config.EnvGitHubToken: ""}, config.Environment{}, ""},
}

func TestReadEnvironment(t *testing.T) {
	for _, tc := range environmentCases {
		t.Run(tc.name, func(t *testing.T) {
			got := config.ReadEnvironment(env(tc.values))
			if got != tc.want {
				t.Fatalf("ReadEnvironment = %+v, want %+v", got, tc.want)
			}
			if got.TokenShadow() != tc.shadow || got.Shadowed() != (tc.shadow != "") {
				t.Fatalf("TokenShadow = %q Shadowed = %v, want %q", got.TokenShadow(), got.Shadowed(), tc.shadow)
			}
		})
	}
}

func TestEnviron(t *testing.T) {
	for _, name := range config.EnvNames() {
		t.Setenv(name, "")
	}
	t.Setenv(config.EnvGitHubToken, "sanitized")
	t.Setenv(config.EnvHome, "/tmp/gh-board-home")
	got := config.Environ()
	want := config.Environment{Home: "/tmp/gh-board-home", GitHubToken: true}
	if got != want {
		t.Fatalf("Environ = %+v, want %+v", got, want)
	}
	if nilGetenv := config.ReadEnvironment(nil); nilGetenv != want {
		t.Fatalf("ReadEnvironment(nil) = %+v, want %+v", nilGetenv, want)
	}
}

func TestEnvNames(t *testing.T) {
	want := []string{config.EnvConfig, config.EnvHome, config.EnvGHToken, config.EnvGitHubToken}
	got := config.EnvNames()
	if len(got) != len(want) {
		t.Fatalf("EnvNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("EnvNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUserHomeDir(t *testing.T) {
	home, err := config.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("UserHomeDir = %q, %v", home, err)
	}
	if want, _ := os.UserHomeDir(); home != want {
		t.Fatalf("UserHomeDir = %q, want %q", home, want)
	}
}
