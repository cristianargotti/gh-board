package config_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

const (
	minimalYAML = "version: 1\nproject: { owner: acme, number: 7 }\n"
	otherYAML   = "version: 1\nproject: { owner: other, number: 1 }\n"
	acmeYAML    = "version: 1\nproject: { owner: beta, number: 9 }\n"
	brokenYAML  = "version: 1\npermissions: all\n"
	defaultYAML = "project: { owner: beta, number: 9 }\n"
	defaultFile = "home/config/default.yml"
)

var (
	beta = domain.ProjectRef{Owner: "beta", Number: 9}
	acme = domain.ProjectRef{Owner: "acme", Number: 7}
)

type loadCase struct {
	name    string
	files   map[string]string
	opts    func(root string) config.LoadOptions
	source  config.Source
	path    string // relative to root, empty for generic
	project domain.ProjectRef
	def     domain.ProjectRef // the recorded default, whatever step won
	cands   []string          // relative to root
	err     error
}

// noFlags are the options of a command run without --project or --config.
func noFlags(root string) config.LoadOptions {
	return config.LoadOptions{Env: env(nil), WorkDir: root, Dirs: homeDirs(root)}
}

// env returns a getenv over a fixed map so the tests never touch the
// process environment.
func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func homeDirs(root string) config.Dirs {
	return config.Paths(filepath.Join(root, "home"))
}

var loadCases = []loadCase{
	{
		name:  "flag wins over env and directory",
		files: map[string]string{"custom.yml": otherYAML, "env.yml": minimalYAML, "repo/.git": "DIR", "repo/board.yml": minimalYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Path: filepath.Join(root, "custom.yml"), Env: env(map[string]string{config.EnvConfig: filepath.Join(root, "env.yml")}), WorkDir: filepath.Join(root, "repo"), Dirs: homeDirs(root)}
		},
		source: config.SourceFlag, path: "custom.yml", project: domain.ProjectRef{Owner: "other", Number: 1},
	},
	{
		name: "flag missing is a usage error",
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Path: filepath.Join(root, "none.yml"), Env: env(nil), WorkDir: root, Dirs: homeDirs(root)}
		},
		err: domain.ErrUsage,
	},
	{
		name: "flag naming a directory is a usage error",
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Path: root, Env: env(nil), WorkDir: root, Dirs: homeDirs(root)}
		},
		err: domain.ErrUsage,
	},
	{
		name:  "env wins over directory",
		files: map[string]string{"env.yml": otherYAML, "repo/.git": "DIR", "repo/board.yml": minimalYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(map[string]string{config.EnvConfig: filepath.Join(root, "env.yml")}), WorkDir: filepath.Join(root, "repo"), Dirs: homeDirs(root)}
		},
		source: config.SourceEnv, path: "env.yml", project: domain.ProjectRef{Owner: "other", Number: 1},
	},
	{
		name: "env missing is a usage error",
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(map[string]string{config.EnvConfig: filepath.Join(root, "none.yml")}), WorkDir: root, Dirs: homeDirs(root)}
		},
		err: domain.ErrUsage,
	},
	{
		name:  "board.yml in the working directory",
		files: map[string]string{"repo/.git": "DIR", "repo/board.yml": minimalYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo"), Dirs: homeDirs(root)}
		},
		source: config.SourceDirectory, path: "repo/board.yml", project: acme,
	},
	{
		name:  "board.yml in a parent inside the git root",
		files: map[string]string{"repo/.git": "DIR", "repo/board.yml": minimalYAML, "repo/sub/dir/.keep": ""},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo", "sub", "dir"), Dirs: homeDirs(root)}
		},
		source: config.SourceDirectory, path: "repo/board.yml", project: acme,
	},
	{
		name:  "git root as a worktree file",
		files: map[string]string{"repo/.git": "gitdir: elsewhere", "repo/board.yml": minimalYAML, "repo/sub/.keep": ""},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo", "sub"), Dirs: homeDirs(root)}
		},
		source: config.SourceDirectory, path: "repo/board.yml", project: acme,
	},
	{
		name:  "board.yml beyond the git root is ignored",
		files: map[string]string{"board.yml": minimalYAML, "repo/.git": "DIR", "repo/sub/.keep": ""},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo", "sub"), Dirs: homeDirs(root)}
		},
		source: config.SourceGeneric,
	},
	{
		name:  "without a git root only the working directory is searched",
		files: map[string]string{"board.yml": minimalYAML, "sub/.keep": ""},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "sub"), Dirs: homeDirs(root)}
		},
		source: config.SourceGeneric,
	},
	{
		name:  "per-user file named after the project",
		files: map[string]string{"home/config/beta-9.yml": otherYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: root, Dirs: homeDirs(root), Project: beta}
		},
		source: config.SourceUser, path: "home/config/beta-9.yml", project: beta,
	},
	{
		name:  "project flag overrides the file",
		files: map[string]string{"repo/.git": "DIR", "repo/board.yml": minimalYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo"), Dirs: homeDirs(root), Project: beta}
		},
		source: config.SourceDirectory, path: "repo/board.yml", project: beta,
	},
	{
		name:  "decode errors name the file",
		files: map[string]string{"repo/.git": "DIR", "repo/board.yml": brokenYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo"), Dirs: homeDirs(root)}
		},
		err: domain.ErrUsage,
	},
	{
		name: "generic mode carries the project flag",
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: root, Dirs: homeDirs(root), Project: beta}
		},
		source: config.SourceGeneric, project: beta,
	},
}

func TestLoad(t *testing.T) {
	for _, tc := range append(append([]loadCase{}, loadCases...), defaultCases...) {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, tc.files)
			got, err := config.Load(tc.opts(root))
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("err = %v, want %v", err, tc.err)
				}
				if strings.Contains(err.Error(), "\n") {
					t.Fatalf("error must be one line: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			checkLoaded(t, root, got, tc)
		})
	}
}

func checkLoaded(t *testing.T, root string, got config.Loaded, tc loadCase) {
	t.Helper()
	if got.Source != tc.source || got.Config == nil {
		t.Fatalf("source = %q config = %v, want %q", got.Source, got.Config, tc.source)
	}
	if got.Config.Project != tc.project || got.Config.Version != config.SupportedVersion {
		t.Fatalf("project = %+v version = %d, want %+v", got.Config.Project, got.Config.Version, tc.project)
	}
	checkDefault(t, root, got, tc)
	if tc.path == "" {
		if got.Path != "" || got.Hash != "" || !got.Generic() {
			t.Fatalf("generic mode must carry no path or hash: %+v", got)
		}
		return
	}
	want := filepath.Join(root, filepath.FromSlash(tc.path))
	if got.Path != want {
		t.Fatalf("path = %q, want %q", got.Path, want)
	}
	sum, err := config.Hash(want)
	if err != nil || got.Hash != sum {
		t.Fatalf("hash = %q, want %q (%v)", got.Hash, sum, err)
	}
}

func TestLoadDefaults(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"repo/.git": "DIR", "repo/board.yml": minimalYAML})
	t.Setenv(config.EnvHome, filepath.Join(root, "home"))
	t.Setenv(config.EnvConfig, "")
	t.Chdir(filepath.Join(root, "repo"))
	got, err := config.Load(config.LoadOptions{})
	if err != nil || got.Source != config.SourceDirectory {
		t.Fatalf("Load with defaults = %+v, %v", got, err)
	}
	if filepath.Base(got.Path) != config.FileName || !filepath.IsAbs(got.Path) {
		t.Fatalf("path = %q", got.Path)
	}
}

var userPathCases = []struct {
	name string
	dir  string
	ref  domain.ProjectRef
	want string
}{
	{"named after the project", "cfg", beta, filepath.Join("cfg", "beta-9.yml")},
	{"zero project", "cfg", domain.ProjectRef{}, ""},
	{"empty dir", "", beta, ""},
	{"owner with separator", "cfg", domain.ProjectRef{Owner: "../x", Number: 1}, ""},
	{"owner is dot dot", "cfg", domain.ProjectRef{Owner: "..", Number: 1}, ""},
}

func TestUserPath(t *testing.T) {
	for _, tc := range userPathCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.UserPath(tc.dir, tc.ref); got != tc.want {
				t.Fatalf("UserPath = %q, want %q", got, tc.want)
			}
		})
	}
}
