// Package config loads board.yml with strict decoding and the resolution
// order of section 5.3. It is the only package that reads configuration
// files, environment variables and the per-OS directories (go-gh).
package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ghconfig "github.com/cli/go-gh/v2/pkg/config"
	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Names the resolution order relies on.
const (
	// AppDir is the directory name under each go-gh directory.
	AppDir = "gh-board"
	// FileName is the per-team file searched from the working directory.
	FileName = "board.yml"
	// EnvConfig names a configuration file explicitly.
	EnvConfig = "GH_BOARD_CONFIG"
	// EnvHome roots config, state and cache under one directory (tests, CI).
	EnvHome = "GH_BOARD_HOME"
)

// Dirs are the kit's directories: config for board.yml copies and agent
// artifacts, state for plans, journal, audit and alerts.json, cache for
// the schema only (section 6.9).
type Dirs struct {
	Config string
	State  string
	Cache  string
}

// Paths returns the kit directories under the go-gh per-OS directories.
// A non-empty override roots the three under it: tests pass a temporary
// directory and users may set GH_BOARD_HOME.
func Paths(override string) Dirs {
	if override != "" {
		return Dirs{
			Config: filepath.Join(override, "config"),
			State:  filepath.Join(override, "state"),
			Cache:  filepath.Join(override, "cache"),
		}
	}
	return Dirs{
		Config: filepath.Join(ghconfig.ConfigDir(), AppDir),
		State:  filepath.Join(ghconfig.StateDir(), AppDir),
		Cache:  filepath.Join(ghconfig.CacheDir(), AppDir),
	}
}

// DefaultPaths returns Paths with the process environment override, so
// that main never reads an environment variable itself.
func DefaultPaths() Dirs {
	return Paths(Environ().Home)
}

// Source says which step of the resolution order produced the configuration.
type Source string

// Sources in resolution order (section 5.3).
const (
	SourceFlag      Source = "flag"      // --config path
	SourceEnv       Source = "env"       // GH_BOARD_CONFIG
	SourceDirectory Source = "directory" // board.yml from the working directory up to the git root
	SourceUser      Source = "user"      // <config dir>/gh-board/<owner>-<number>.yml
	SourceDefault   Source = "default"   // no file: the project recorded by gh board use
	SourceGeneric   Source = "generic"   // no file: generic mode
)

// LoadOptions are the inputs of Load. Zero values mean "use the process
// environment": Env defaults to os.Getenv and WorkDir to the current
// directory.
type LoadOptions struct {
	// Path is the --config flag.
	Path string
	// Env reads environment variables; nil means os.Getenv.
	Env func(string) string
	// WorkDir is where the upward search for board.yml starts.
	WorkDir string
	// Dirs are the user directories; zero means Paths(Env(EnvHome)).
	Dirs Dirs
	// Project is the --project override, applied over any file.
	Project domain.ProjectRef
}

// Loaded is the configuration in use with its provenance. doctor prints
// Path and Hash so that a poisoned file is visible (F6), and the default
// with where it came from.
type Loaded struct {
	Config *domain.Config
	Path   string
	Source Source
	Hash   string
	// Default is the project gh board use recorded, zero when none.
	Default Default
	// Candidates are the per-user files the resolution could not choose
	// between: two or more exist and no default names one.
	Candidates []string
}

// Generic reports whether no file was found and generic mode applies.
func (l Loaded) Generic() bool { return l.Source == SourceGeneric || l.Source == SourceDefault }

// SupportedVersion is the only board.yml version this kit reads.
const SupportedVersion = 1

// Decode parses board.yml strictly: unknown keys (capability names
// included), unknown alert ids, unknown digest metrics and an unsupported
// version are errors (section 5.2). It does not check names against the
// board; Validate does.
func Decode(data []byte) (*domain.Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cfg domain.Config
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("board.yml is empty: %w", domain.ErrUsage)
		}
		return nil, fmt.Errorf("board.yml: %s: %w", oneLine(err), domain.ErrUsage)
	}
	if cfg.Version != SupportedVersion {
		return nil, usage("version %d is not supported, expected %d", cfg.Version, SupportedVersion)
	}
	if err := checkIdentifiers(cfg); err != nil {
		return nil, err
	}
	if err := checkShape(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// oneLine flattens the multi-line messages of the YAML decoder.
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

// checkIdentifiers rejects alert ids and digest metrics the kit does not
// know, so that a typo never silently disables a rule.
func checkIdentifiers(cfg domain.Config) error {
	ids := make([]string, 0, len(cfg.Alerts))
	for id := range cfg.Alerts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !domain.Contains(domain.AlertRuleIDs, id) {
			return usage("unknown alert %q", id)
		}
	}
	for _, m := range cfg.Digest.Metrics {
		if !domain.Contains(domain.DigestMetrics, m) {
			return usage("unknown digest metric %q", m)
		}
	}
	for _, role := range cfg.Tidy.InheritFromParent {
		if !domain.Contains(domain.TidyInheritable, role) {
			return usage("tidy.inherit_from_parent %q is not lane or epic", role)
		}
	}
	return nil
}

// Mismatch is one name in board.yml that the discovered schema lacks.
type Mismatch struct {
	// Path is the YAML path, for example capabilities.lane.field.
	Path string
	// Value is the name that does not exist on the board.
	Value string
	// Reason explains the mismatch, with a "did you mean" when close.
	Reason string
	// Commands lists the verbs that depend on this mapping.
	Commands []string
}

// Hash returns the SHA-256 hex digest of the file at path.
func Hash(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // the path is the configuration file the user chose
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
