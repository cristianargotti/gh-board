package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

type readDoctorCase struct {
	name  string
	args  []string
	setup func(t *testing.T, deps *Deps, fake *readFakeReader)
	want  []string
}

var readDoctorCases = []readDoctorCase{
	{"healthy", nil, func(*testing.T, *Deps, *readFakeReader) {}, []string{"Can write:   yes", "no agent artifacts recorded", "watch has not run yet", "does not expose release checksums"}},
	{"dev build", nil, func(_ *testing.T, deps *Deps, _ *readFakeReader) { deps.Version = "dev" }, []string{"development build: no release checksum"}},
	{"no reader", nil, func(_ *testing.T, deps *Deps, _ *readFakeReader) { deps.Reader = nil }, []string{"identity: no GitHub reader", "project: no GitHub reader", "no GitHub reader configured"}},
	{"viewer fails", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.fail["Viewer"] = errors.New("boom") }, []string{"viewer unavailable: boom"}},
	{"project fails", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.fail["DiscoverProject"] = errors.New("boom") }, []string{"unreachable: boom"}},
	{"permission fails", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.fail["ViewerPermission"] = errors.New("boom") }, []string{"permission unavailable: boom"}},
	{"read only", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.permission = domain.PermissionRead }, []string{"no write permission on acme/app (READ)"}},
	{"labels fail", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.fail["RepositoryLabels"] = errors.New("boom") }, []string{"labels not checked: boom"}},
	{"types fail", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.fail["IssueTypes"] = errors.New("boom") }, []string{"issue types not checked: boom"}},
	{"issue fields fail", nil, func(_ *testing.T, deps *Deps, f *readFakeReader) {
		deps.Reader = &readFakeIssueFields{readFakeReader: f, err: errors.New("boom")}
	}, []string{"issue fields not checked: boom"}},
	{"issue fields listed", nil, func(_ *testing.T, deps *Deps, f *readFakeReader) {
		deps.Reader = &readFakeIssueFields{readFakeReader: f, fields: []domain.Field{{Name: "Start date"}, {Name: "Target date"}}}
	}, []string{"Can write:   yes"}},
	{"shadowed env", nil, func(t *testing.T, _ *Deps, _ *readFakeReader) { t.Setenv(config.EnvGHToken, "x") }, []string{"GH_TOKEN takes precedence", "shadows the stored gh credentials"}},
	{"shadowed viewer", nil, func(_ *testing.T, _ *Deps, f *readFakeReader) { f.viewer.TokenSource = config.EnvGitHubToken }, []string{"identity: an environment token shadows"}},
	{"mismatch", nil, func(_ *testing.T, deps *Deps, _ *readFakeReader) {
		deps.Config.Config.Capabilities.Lane = &domain.FieldCapability{Field: "Faixa"}
	}, []string{"capabilities.lane.field", "Faixa", "board.yml capabilities.lane.field"}},
	{"bad config file", []string{"--config", filepath.Join("testdata", "nope.yml"), "doctor"}, func(_ *testing.T, deps *Deps, _ *readFakeReader) {
		deps.Config = nil
	}, []string{"configuration: --config names"}},
	{"no project", nil, func(_ *testing.T, deps *Deps, _ *readFakeReader) {
		deps.Config = &config.Loaded{Config: &domain.Config{Version: config.SupportedVersion}, Source: config.SourceGeneric}
	}, []string{"project: no project selected", "gh board use owner/number", "repository not configured", "no default project recorded"}},
	{"default recorded", nil, func(t *testing.T, deps *Deps, _ *readFakeReader) {
		deps.Config = nil
		readWriteFile(t, config.DefaultPath(deps.Dirs.Config), string(config.EncodeDefault(domain.ProjectRef{Owner: "acme", Number: 7})))
	}, []string{"Source:           default", "Default project:  acme/7", "Default file:     ", "Mode:             generic"}},
	{"candidates without a default", nil, func(t *testing.T, deps *Deps, _ *readFakeReader) {
		deps.Config = nil
		readWriteFile(t, filepath.Join(deps.Dirs.Config, "acme-7.yml"), readFixtureYAML)
		readWriteFile(t, filepath.Join(deps.Dirs.Config, "other-1.yml"), "version: 1\n")
	}, []string{"per-user files without a default: ", "project: no project selected: 2 per-user files (acme-7.yml, other-1.yml)"}},
	{"bad repository", nil, func(_ *testing.T, deps *Deps, _ *readFakeReader) { deps.Config.Config.Repository = "nope" }, []string{"repository: repository \"nope\""}},
	{"damaged receipt", nil, func(t *testing.T, deps *Deps, _ *readFakeReader) {
		readWriteFile(t, filepath.Join(deps.Dirs.Config, "agents", "claude-user.json"), "{")
	}, []string{"agent artifacts unreadable"}},
	{"artifacts ok", nil, func(t *testing.T, deps *Deps, _ *readFakeReader) {
		readInstallReceipt(t, deps)
	}, []string{"  ok", "Problems\n  none"}},
	{"binary path fails", nil, func(t *testing.T, _ *Deps, _ *readFakeReader) {
		readStubExecutableError(t, errors.New("no exe"))
	}, []string{"binary path unavailable: no exe"}},
	{"binary unreadable", nil, func(t *testing.T, _ *Deps, _ *readFakeReader) {
		readStubExecutable(t, filepath.Join("testdata", "nope"))
	}, []string{"binary unreadable"}},
	{"damaged alerts", nil, func(t *testing.T, deps *Deps, _ *readFakeReader) {
		readWriteFile(t, filepath.Join(deps.Dirs.State, "alerts.json"), "{")
	}, []string{"alerts.json is damaged"}},
	{"unreadable alerts", nil, func(t *testing.T, deps *Deps, _ *readFakeReader) {
		if err := os.MkdirAll(filepath.Join(deps.Dirs.State, "alerts.json"), 0o700); err != nil {
			t.Fatal(err)
		}
	}, []string{"alerts.json unreadable"}},
}

func TestReadDoctor(t *testing.T) {
	for _, tc := range readDoctorCases {
		t.Run(tc.name, func(t *testing.T) {
			readPinEnvironment(t)
			fake := readNewFakeReader()
			deps, out := readTestDeps(t, fake)
			tc.setup(t, deps, fake)
			args := tc.args
			if args == nil {
				args = []string{"doctor"}
			}
			if err := readRun(t, deps, args...); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output lacks %q:\n%s", want, out.String())
				}
			}
		})
	}
}

func readWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := audit.WriteAtomic(path, []byte(content)); err != nil {
		t.Fatal(err)
	}
}

// readInstallReceipt records, through the receipt writer of the
// installers, one guard file and the hook fragment the guard generates
// for this operating system, so doctor verifies the hash of each and the
// binary the hook command names, whatever the path form of the platform.
func readInstallReceipt(t *testing.T, deps *Deps) {
	t.Helper()
	file := filepath.Join(deps.Dirs.Config, "artifact.txt")
	readWriteFile(t, file, "guard\n")
	hook, err := autoHookFragment(guard.CodexHook, false, "codex hook")
	if err != nil {
		t.Fatal(err)
	}
	content, err := autoJSONEncode(hook)
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(deps.Dirs.Config, "hooks.json")
	readWriteFile(t, hooks, string(content))
	receipt := autoReceipt{Agent: guard.AgentCodex, Scope: autoScopeUser, Version: deps.Version, InstalledAt: deps.Now(), Entries: []autoReceiptEntry{
		{Kind: autoKindGuard, Type: autoTypeFile, Path: file, Created: true, SHA256: guard.SHA256([]byte("guard\n"))},
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: hooks, Created: true, Delta: hook, SHA256: guard.SHA256(content)},
	}}
	if err := autoWriteReceipt(autoReceiptPath(deps.Dirs, guard.AgentCodex, autoScopeUser), receipt); err != nil {
		t.Fatal(err)
	}
}

func readStubExecutableError(t *testing.T, err error) {
	t.Helper()
	previous := readExecutable
	readExecutable = func() (string, error) { return "", err }
	t.Cleanup(func() { readExecutable = previous })
}

var readChecksumCases = []struct {
	name string
	sum  string
	err  error
	want string
}{
	{"verified", "", nil, "Verified:          yes"},
	{"differs", "deadbeef", nil, "checksum differs from the release checksum"},
	{"error", "", errors.New("offline"), "release checksum unavailable: offline"},
}

func TestReadDoctorReleaseChecksum(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "gh-board")
	readWriteFile(t, binary, readHomeBinary)
	actual, err := readFileSHA256(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range readChecksumCases {
		t.Run(tc.name, func(t *testing.T) {
			readPinEnvironment(t)
			readStubExecutable(t, binary)
			fake := &readFakeChecksums{readFakeReader: readNewFakeReader(), sum: tc.sum, err: tc.err}
			if fake.sum == "" && tc.err == nil {
				fake.sum = actual
			}
			deps, out := readTestDeps(t, fake)
			if err := readRun(t, deps, "doctor"); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output lacks %q:\n%s", tc.want, out.String())
			}
			if !strings.HasPrefix(fake.asset, "gh-board_1.2.3_") {
				t.Fatalf("asset = %q", fake.asset)
			}
		})
	}
}

func TestReadLogEntries(t *testing.T) {
	deps, out := readTestDeps(t, readNewFakeReader())
	entries := []audit.Entry{
		{At: readFixtureNow.AddDate(0, 0, -1), Actor: "ana", Command: "assign acme/app#3 bruno", Targets: []string{"I_kwDOAbc003"}, Result: "ok"},
		{At: readFixtureNow.AddDate(0, 0, -30), Actor: "ana", Command: "comment acme/app#1", Result: "ok"},
	}
	for _, e := range entries {
		if err := audit.Append(deps.Dirs.State, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := readRun(t, deps, "log", "--since", "2d"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Entries:    1") || !strings.Contains(out.String(), "assign acme/app#3 bruno") {
		t.Fatalf("output = %s", out.String())
	}
	if err := os.RemoveAll(audit.Path(deps.Dirs.State)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(audit.Path(deps.Dirs.State), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := readRun(t, deps, "log"); err == nil {
		t.Fatal("a directory in place of the log must fail")
	}
}
