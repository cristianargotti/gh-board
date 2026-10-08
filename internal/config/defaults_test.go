package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestDefaultRoundTrip(t *testing.T) {
	data := config.EncodeDefault(acme)
	text := string(data)
	if !strings.HasPrefix(text, "# Default project of gh board") || !strings.Contains(text, "gh board use --clear") {
		t.Fatalf("EncodeDefault = %q", text)
	}
	assertNoDashes(t, text)
	got, err := config.DecodeDefault(data)
	if err != nil || got != acme {
		t.Fatalf("DecodeDefault = %+v, %v", got, err)
	}
}

var decodeDefaultInvalid = map[string]string{
	"empty":          "",
	"unknown key":    "project: { owner: beta, number: 9 }\nversion: 1\n",
	"missing":        "# nothing\n",
	"owner missing":  "project: { number: 7 }\n",
	"number zero":    "project: { owner: beta, number: 0 }\n",
	"owner slash":    "project: { owner: a/b, number: 7 }\n",
	"owner spaces":   "project: { owner: ' beta', number: 9 }\n",
	"malformed":      "project: [\n",
	"wrong type":     "project: { owner: beta, number: seven }\n",
	"project scalar": "project: beta/9\n",
}

func TestDecodeDefaultInvalid(t *testing.T) {
	for name, in := range decodeDefaultInvalid {
		t.Run(name, func(t *testing.T) {
			_, err := config.DecodeDefault([]byte(in))
			if !errors.Is(err, domain.ErrUsage) {
				t.Fatalf("expected ErrUsage, got %v", err)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Fatalf("error must be one line: %q", err)
			}
		})
	}
}

func TestReadDefault(t *testing.T) {
	if config.DefaultPath("") != "" {
		t.Fatal("no config directory means no default path")
	}
	if got, err := config.ReadDefault(""); err != nil || !got.IsZero() {
		t.Fatalf("ReadDefault without a directory = %+v, %v", got, err)
	}
	dir := filepath.Join(t.TempDir(), "config")
	if got, err := config.ReadDefault(dir); err != nil || !got.IsZero() {
		t.Fatalf("ReadDefault of a missing directory = %+v, %v", got, err)
	}
	writeFiles(t, dir, map[string]string{config.DefaultFileName: string(config.EncodeDefault(beta))})
	got, err := config.ReadDefault(dir)
	if err != nil || got.Project != beta || got.Path != config.DefaultPath(dir) {
		t.Fatalf("ReadDefault = %+v, %v", got, err)
	}
	writeFiles(t, dir, map[string]string{config.DefaultFileName: "project: 1\n"})
	_, err = config.ReadDefault(dir)
	if !errors.Is(err, domain.ErrUsage) || !strings.Contains(err.Error(), config.DefaultPath(dir)) || !strings.Contains(err.Error(), "gh board use --clear") {
		t.Fatalf("a damaged default must name the file and the way out: %v", err)
	}
	if err := os.Remove(config.DefaultPath(dir)); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{config.DefaultFileName: "DIR"})
	if _, err := config.ReadDefault(dir); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("a directory in place of the default must fail: %v", err)
	}
}

func TestUserFiles(t *testing.T) {
	if config.UserFiles("") != nil || config.UserFiles(filepath.Join(t.TempDir(), "none")) != nil {
		t.Fatal("no directory means no files")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"other-1.yml": otherYAML, "beta-9.yml": acmeYAML, config.DefaultFileName: defaultYAML,
		"old.yaml": "", "agents/claude-user.json": "{}", "nested/x.yml": "DIR",
	})
	got := config.UserFiles(dir)
	want := []string{filepath.Join(dir, "beta-9.yml"), filepath.Join(dir, "other-1.yml")}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("UserFiles = %v, want %v", got, want)
	}
}

var fileNameCases = []struct {
	name string
	want domain.ProjectRef
	ok   bool
}{
	{"beta-9.yml", beta, true},
	{filepath.Join("home", "config", "acme-7.yml"), acme, true},
	{"notes.yml", domain.ProjectRef{}, false},
	{"-7.yml", domain.ProjectRef{}, false},
	{"beta-x.yml", domain.ProjectRef{}, false},
	{"beta-0.yml", domain.ProjectRef{}, false},
	{"beta-9", beta, true},
}

func TestProjectFromFileName(t *testing.T) {
	for _, tc := range fileNameCases {
		got, ok := config.ProjectFromFileName(tc.name)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ProjectFromFileName(%q) = %+v, %v", tc.name, got, ok)
		}
	}
}
