package config

import "os"

// Token variables gh honors. Their values are never read by the kit: only
// their presence matters, for the shadowing report of section 6.6.
const (
	// EnvGHToken takes precedence over EnvGitHubToken in gh.
	EnvGHToken = "GH_TOKEN"
	// EnvGitHubToken is the token variable GitHub Actions sets.
	EnvGitHubToken = "GITHUB_TOKEN" //nolint:gosec // the variable name, never a credential
)

// EnvNames lists every environment variable the kit reads, in precedence
// order where that applies, so doctor and the docs stay accurate.
func EnvNames() []string {
	return []string{EnvConfig, EnvHome, EnvGHToken, EnvGitHubToken}
}

// Environment is what the kit learned from the process environment. It
// carries paths and presence flags, never a token value.
type Environment struct {
	// ConfigPath is GH_BOARD_CONFIG, empty when unset.
	ConfigPath string
	// Home is GH_BOARD_HOME, the override of the per-OS directories.
	Home string
	// GHToken reports whether GH_TOKEN is set to a non-empty value.
	GHToken bool
	// GitHubToken reports whether GITHUB_TOKEN is set to a non-empty value.
	GitHubToken bool
}

// ReadEnvironment reads the variables through getenv; nil means os.Getenv.
func ReadEnvironment(getenv func(string) string) Environment {
	if getenv == nil {
		getenv = os.Getenv
	}
	return Environment{
		ConfigPath:  getenv(EnvConfig),
		Home:        getenv(EnvHome),
		GHToken:     getenv(EnvGHToken) != "",
		GitHubToken: getenv(EnvGitHubToken) != "",
	}
}

// Environ reads the process environment.
func Environ() Environment {
	return ReadEnvironment(nil)
}

// TokenShadow names the variable whose token shadows the credentials gh
// stored: GH_TOKEN wins over GITHUB_TOKEN, as in gh. Empty when neither
// is set.
func (e Environment) TokenShadow() string {
	switch {
	case e.GHToken:
		return EnvGHToken
	case e.GitHubToken:
		return EnvGitHubToken
	default:
		return ""
	}
}

// Shadowed reports whether an environment token takes precedence over the
// keyring token, which doctor and every write report (section 6.6).
func (e Environment) Shadowed() bool {
	return e.TokenShadow() != ""
}

// UserHomeDir is the home directory of the person, where the three agents
// keep their settings and the schedulers their jobs. The kit resolves it
// here only, so that every directory read stays in this package.
func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
