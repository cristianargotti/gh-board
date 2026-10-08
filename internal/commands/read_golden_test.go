package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

var readUpdateGolden = flag.Bool("update-read-golden", false, "rewrite the golden files of the read commands")

// readGoldenHome is the generated home of the doctor and log cases: a
// short relative path, so the tables align the same on every machine. The
// test creates it and removes it.
var readGoldenHome = filepath.Join("testdata", "read", ".home")

var readGoldenCases = []struct {
	name string
	args []string
}{
	{"context", []string{"context"}},
	{"context_for", []string{"context", "--for", "bruno", "--budget", "300"}},
	{"status", []string{"status"}},
	{"me", []string{"me"}},
	{"item", []string{"item", "acme/app#3"}},
	{"item_short", []string{"item", "#4"}},
	{"list", []string{"list"}},
	{"list_filters", []string{"list", "--assignee", "ana", "--status", "IN PROGRESS", "--sprint", "current"}},
	{"list_limit", []string{"list", "--limit", "2"}},
	{"list_rules", []string{"list", "--all", "--overdue"}},
	{"epics", []string{"epics"}},
	{"roadmap", []string{"roadmap", "--months", "4"}},
	{"sprint_current", []string{"sprint", "current"}},
	{"sprint_next", []string{"sprint", "next"}},
	{"attention", []string{"attention"}},
	{"search", []string{"search", "login"}},
	{"schema", []string{"schema"}},
	{"doctor", []string{"doctor"}},
	{"log", []string{"log", "--since", "30d"}},
	{"digest", []string{"digest", "print"}},
	{"digest_week", []string{"digest", "print", "--week", "2026-W40"}},
	{"standup", []string{"standup"}},
	{"standup_for", []string{"standup", "--for", "bruno"}},
}

var readGoldenFormats = []string{"table", "compact", "md", "json"}

func TestReadGolden(t *testing.T) {
	for _, tc := range readGoldenCases {
		for _, format := range readGoldenFormats {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				deps, out, withHome := readGoldenDeps(t, tc.name)
				args := append(append([]string{}, tc.args...), "--format", format)
				if err := readRun(t, deps, args...); err != nil {
					t.Fatalf("%v: %v", args, err)
				}
				got := readNormalizeSeparators(out.Bytes(), withHome)
				readCheckGolden(t, tc.name+"."+format, got)
				if format == "json" && !json.Valid(got) {
					t.Fatalf("invalid json: %s", got)
				}
			})
		}
	}
}

// readGoldenDeps pins everything that could vary between runs: the home
// directory and its files, the environment and the running binary.
func readGoldenDeps(t *testing.T, name string) (*Deps, *bytes.Buffer, bool) {
	t.Helper()
	deps, out := readTestDeps(t, readNewFakeReader())
	withHome := name == "doctor" || name == "log"
	if withHome {
		home := readMakeHome(t)
		deps.Dirs = config.Paths(home)
		readPinEnvironment(t)
		readStubExecutable(t, filepath.Join(home, "bin", "gh-board"))
	}
	return deps, out, withHome
}

// Fixed contents of the generated home. The hashes are computed from the
// bytes written, so a CRLF checkout cannot change them.
const (
	readHomeDeny   = `{"permissions":{"deny":["Bash(gh project delete *)","Bash(gh board apply *)"]}}` + "\n"
	readHomeHooks  = `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"gh board guard check --agent claude"}]}]}}` + "\n"
	readHomeBinary = "placeholder binary for the doctor tests\n"
	readHomeAlerts = `{"generated_at":"2026-10-08T11:30:00Z","project":{"owner":"acme","number":7},"summary":"Sprint 5 · 11 dias · 1 atrasada",` +
		`"alerts":[{"rule_id":"overdue","severity":"critical","item":{"node_id":"I_kwDOAbc003","project_item_id":"PVTI_lADOAbc003",` +
		`"repository":"acme/app","number":3,"title":"Corrigir login SSO","url":"https://github.com/acme/app/issues/3"},` +
		`"message":"Atrasada há 7 dias (alvo 01/10/2026)","first_seen":"2026-10-02T08:00:00Z"}]}` + "\n"
	readHomeAudit = `{"at":"2026-10-01T10:00:00Z","actor":"ana","host":"github.com","project":{"owner":"acme","number":7},` +
		`"command":"move acme/app#3 IN PROGRESS","reason":"inicio do sprint","targets":["I_kwDOAbc003"],"result":"ok","dry_run":false}` + "\n" +
		`{"at":"2026-10-07T09:00:00Z","actor":"ana","host":"github.com","project":{"owner":"acme","number":7},` +
		`"command":"close acme/app#6","targets":["I_kwDOAbc006"],"result":"failed","error":"drift detected","dry_run":true,"plan_id":"20261007T090000Z-ab12"}` + "\n"
)

// readMakeHome builds a home with one receipt whose artifacts are intact,
// modified and missing, an alert state and an audit log.
func readMakeHome(t *testing.T) string {
	t.Helper()
	home := readGoldenHome
	if err := os.RemoveAll(home); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	dirs := config.Paths(home)
	deny := filepath.Join(home, "artifacts", "deny.json")
	hooks := filepath.Join(home, "artifacts", "hooks.json")
	missing := filepath.Join(home, "artifacts", "missing.md")
	files := map[string]string{
		deny: readHomeDeny, hooks: readHomeHooks, filepath.Join(home, "bin", "gh-board"): readHomeBinary,
		filepath.Join(dirs.State, "alerts.json"): readHomeAlerts, filepath.Join(dirs.State, audit.FileName): readHomeAudit,
	}
	for path, content := range files {
		readWriteFile(t, path, content)
	}
	receipt := map[string]any{
		"agent": "claude", "scope": "user", "strict": false, "version": "1.2.3", "installed_at": "2026-10-01T09:00:00Z",
		"artifacts": []map[string]any{
			{"kind": "guard", "type": "json", "path": hooks, "created": false, "sha256": strings.Repeat("0", 64)},
			{"kind": "guard", "type": "file", "path": deny, "created": true, "sha256": guard.SHA256([]byte(readHomeDeny))},
			{"kind": "skill", "type": "file", "path": missing, "created": true, "sha256": strings.Repeat("1", 64)},
		},
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	readWriteFile(t, filepath.Join(dirs.Config, "agents", "claude-user.json"), string(data))
	return home
}

// readNormalizeSeparators turns the path separators of the generated home
// into slashes, JSON-escaped ones included, so the same golden serves
// every operating system.
func readNormalizeSeparators(out []byte, withHome bool) []byte {
	if !withHome {
		return out
	}
	normalized := bytes.ReplaceAll(out, []byte(`\\`), []byte(`/`))
	return bytes.ReplaceAll(normalized, []byte(`\`), []byte(`/`))
}

func readPinEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range config.EnvNames() {
		t.Setenv(name, "")
	}
}

func readStubExecutable(t *testing.T, path string) {
	t.Helper()
	previous := readExecutable
	readExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { readExecutable = previous })
}

func readCheckGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "read", name+".golden")
	if *readUpdateGolden {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update-read-golden to create it)", err)
	}
	// A checkout that converted line endings must not fail the comparison.
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the golden file\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestReadGoldenFilesCarryNoDashesOrPaths(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "read"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", "read", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(string(data), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains a dash", e.Name())
		}
		if strings.Contains(string(data), os.TempDir()) {
			t.Errorf("%s contains a temporary path", e.Name())
		}
	}
}
