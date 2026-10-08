package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Load resolves and decodes the configuration in the order of section 5.3:
// --config, GH_BOARD_CONFIG, board.yml from the working directory up to the
// git root, the per-user file named after the --project override, the
// default recorded by gh board use, a single per-user file, then generic
// mode. The --project override is applied last, over any file. Generic
// mode is not an error: it returns an empty configuration that carries
// only the project the steps selected, so callers never nil-check Config.
func Load(opts LoadOptions) (Loaded, error) {
	opts = opts.withDefaults()
	def, err := ReadDefault(opts.Dirs.Config)
	if err != nil {
		return Loaded{}, err
	}
	st, err := resolve(opts, def)
	if err != nil {
		return Loaded{}, err
	}
	loaded := Loaded{Config: &domain.Config{Version: SupportedVersion, Project: st.project}, Source: st.source}
	if st.path != "" {
		if loaded, err = readFile(st.path, st.source); err != nil {
			return Loaded{}, err
		}
		loaded.nameProject()
	}
	loaded.Default, loaded.Candidates = def, st.candidates
	if !opts.Project.IsZero() {
		loaded.Config.Project = opts.Project
	}
	return loaded, nil
}

// nameProject fills the project of a per-user file that carries none from
// its name, which is <owner>-<number>.yml by convention.
func (l *Loaded) nameProject() {
	if l.Source != SourceUser || !l.Config.Project.IsZero() {
		return
	}
	if ref, ok := ProjectFromFileName(l.Path); ok {
		l.Config.Project = ref
	}
}

// withDefaults fills the zero values with the process environment. The
// working directory is made absolute so that the upward walk ends at the
// filesystem root instead of at ".".
func (o LoadOptions) withDefaults() LoadOptions {
	if o.Env == nil {
		o.Env = os.Getenv
	}
	if o.WorkDir == "" {
		if wd, err := os.Getwd(); err == nil {
			o.WorkDir = wd
		}
	}
	if abs, err := filepath.Abs(o.WorkDir); err == nil {
		o.WorkDir = abs
	}
	if o.Dirs == (Dirs{}) {
		o.Dirs = Paths(o.Env(EnvHome))
	}
	return o
}

// step is the outcome of the resolution: the file in use, or in generic
// mode the project the step selected and the per-user files the last step
// could not choose between.
type step struct {
	path       string
	source     Source
	project    domain.ProjectRef
	candidates []string
}

// resolve walks the steps of section 5.3. Explicit paths (flag,
// environment) must exist: a typo there is a usage error, never a silent
// fall through to another file (F6).
func resolve(opts LoadOptions, def Default) (step, error) {
	if opts.Path != "" {
		return requireFile(opts.Path, SourceFlag, "--config")
	}
	if p := opts.Env(EnvConfig); p != "" {
		return requireFile(p, SourceEnv, EnvConfig)
	}
	if p := findInTree(opts.WorkDir); p != "" {
		return step{path: p, source: SourceDirectory}, nil
	}
	if !opts.Project.IsZero() {
		return userStep(opts.Dirs.Config, opts.Project, SourceGeneric), nil
	}
	if !def.IsZero() {
		return userStep(opts.Dirs.Config, def.Project, SourceDefault), nil
	}
	return singleUserFile(opts.Dirs.Config), nil
}

// requireFile checks an explicitly named configuration file.
func requireFile(path string, source Source, origin string) (step, error) {
	info, err := os.Stat(path)
	if err != nil {
		return step{}, fmt.Errorf("%s names %q, which cannot be read: %s: %w", origin, path, oneLine(err), domain.ErrUsage)
	}
	if info.IsDir() {
		return step{}, fmt.Errorf("%s names %q, which is a directory, not a board.yml: %w", origin, path, domain.ErrUsage)
	}
	return step{path: path, source: source}, nil
}

// userStep takes the per-user file named after the project when it
// exists, otherwise generic mode with that project under the source that
// selected it.
func userStep(configDir string, ref domain.ProjectRef, fallback Source) step {
	if p := UserPath(configDir, ref); isFile(p) {
		return step{path: p, source: SourceUser}
	}
	return step{source: fallback, project: ref}
}

// singleUserFile is the last step before generic mode: exactly one
// per-user file serves on its own; two or more stay listed as candidates,
// so the error that asks for --project can name them.
func singleUserFile(configDir string) step {
	files := UserFiles(configDir)
	switch len(files) {
	case 0:
		return step{source: SourceGeneric}
	case 1:
		return step{path: files[0], source: SourceUser}
	default:
		return step{source: SourceGeneric, candidates: files}
	}
}

// findInTree looks for board.yml from workDir upwards and stops at the git
// root. Without a git root only workDir itself is searched, so a file left
// in an unrelated parent directory is never picked up.
func findInTree(workDir string) string {
	if workDir == "" {
		return ""
	}
	root := gitRoot(workDir)
	dir := filepath.Clean(workDir)
	for {
		if candidate := filepath.Join(dir, FileName); isFile(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if root == "" || dir == root || parent == dir {
			return ""
		}
		dir = parent
	}
}

// gitRoot returns the nearest ancestor of dir (dir included) that holds a
// .git entry, which may be a directory or a worktree file; "" when none.
func gitRoot(dir string) string {
	for d := filepath.Clean(dir); ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// UserPath is the per-user configuration file of one project:
// <config dir>/<owner>-<number>.yml. It is empty when the project is not
// known, because nothing can name the file then, and when the owner could
// escape the directory.
func UserPath(configDir string, ref domain.ProjectRef) string {
	if ref.IsZero() || configDir == "" || strings.ContainsAny(ref.Owner, `/\`) || ref.Owner == ".." {
		return ""
	}
	return filepath.Join(configDir, fmt.Sprintf("%s-%d.yml", ref.Owner, ref.Number))
}

// isFile reports whether path exists and is not a directory.
func isFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// readFile decodes one file and hashes the bytes it decoded, so the hash
// doctor prints is the hash of the content in use.
func readFile(path string, source Source) (Loaded, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the resolution order the user controls
	if err != nil {
		return Loaded{}, fmt.Errorf("%s: %s: %w", path, oneLine(err), domain.ErrUsage)
	}
	cfg, err := Decode(data)
	if err != nil {
		return Loaded{}, fmt.Errorf("%s: %w", path, err)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	sum := sha256.Sum256(data)
	return Loaded{Config: cfg, Path: path, Source: source, Hash: hex.EncodeToString(sum[:])}, nil
}
