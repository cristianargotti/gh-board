package config_test

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// defaultCases cover the two steps after the board.yml walk: the recorded
// default, then a single per-user file.
var defaultCases = []loadCase{
	{
		name:   "default recorded and its per-user file",
		files:  map[string]string{defaultFile: defaultYAML, "home/config/beta-9.yml": acmeYAML},
		opts:   noFlags,
		source: config.SourceUser, path: "home/config/beta-9.yml", project: beta, def: beta,
	},
	{
		name:   "default recorded without a per-user file",
		files:  map[string]string{defaultFile: defaultYAML},
		opts:   noFlags,
		source: config.SourceDefault, project: beta, def: beta,
	},
	{
		name:   "default wins over a single per-user file of another project",
		files:  map[string]string{defaultFile: defaultYAML, "home/config/other-1.yml": otherYAML},
		opts:   noFlags,
		source: config.SourceDefault, project: beta, def: beta,
	},
	{
		name:  "project flag wins over the default",
		files: map[string]string{defaultFile: defaultYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: root, Dirs: homeDirs(root), Project: acme}
		},
		source: config.SourceGeneric, project: acme, def: beta,
	},
	{
		name:  "board.yml in the tree wins over the default",
		files: map[string]string{defaultFile: defaultYAML, "repo/.git": "DIR", "repo/board.yml": minimalYAML},
		opts: func(root string) config.LoadOptions {
			return config.LoadOptions{Env: env(nil), WorkDir: filepath.Join(root, "repo"), Dirs: homeDirs(root)}
		},
		source: config.SourceDirectory, path: "repo/board.yml", project: acme, def: beta,
	},
	{
		name:   "a single per-user file serves on its own",
		files:  map[string]string{"home/config/other-1.yml": otherYAML},
		opts:   noFlags,
		source: config.SourceUser, path: "home/config/other-1.yml", project: domain.ProjectRef{Owner: "other", Number: 1},
	},
	{
		name:   "a per-user file without a project is named after it",
		files:  map[string]string{"home/config/acme-7.yml": "version: 1\n"},
		opts:   noFlags,
		source: config.SourceUser, path: "home/config/acme-7.yml", project: acme,
	},
	{
		name:   "a per-user file named oddly selects no project",
		files:  map[string]string{"home/config/notes.yml": "version: 1\n"},
		opts:   noFlags,
		source: config.SourceUser, path: "home/config/notes.yml",
	},
	{
		name:   "two per-user files keep asking and are named",
		files:  map[string]string{"home/config/beta-9.yml": acmeYAML, "home/config/other-1.yml": otherYAML, "home/config/agents/x.yml": "", "home/config/old.yaml": ""},
		opts:   noFlags,
		source: config.SourceGeneric, cands: []string{"home/config/beta-9.yml", "home/config/other-1.yml"},
	},
	{
		name:  "a damaged default is a usage error",
		files: map[string]string{defaultFile: "project: nope\n"},
		opts:  noFlags,
		err:   domain.ErrUsage,
	},
	{
		name:  "a directory in place of the default is a usage error",
		files: map[string]string{defaultFile: "DIR"},
		opts:  noFlags,
		err:   domain.ErrUsage,
	},
}

// checkDefault compares the recorded default and the candidates, which
// doctor and the "no project selected" error print.
func checkDefault(t *testing.T, root string, got config.Loaded, tc loadCase) {
	t.Helper()
	if got.Default.Project != tc.def {
		t.Fatalf("Default = %+v, want %+v", got.Default, tc.def)
	}
	if !tc.def.IsZero() && got.Default.Path != filepath.Join(root, filepath.FromSlash(defaultFile)) {
		t.Fatalf("Default.Path = %q", got.Default.Path)
	}
	if len(got.Candidates) != len(tc.cands) {
		t.Fatalf("Candidates = %v, want %v", got.Candidates, tc.cands)
	}
	for i, c := range tc.cands {
		if got.Candidates[i] != filepath.Join(root, filepath.FromSlash(c)) {
			t.Fatalf("Candidates[%d] = %q, want %q", i, got.Candidates[i], c)
		}
	}
}
