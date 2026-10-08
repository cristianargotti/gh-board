package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// DefaultFileName is the file gh board use writes under the config
// directory: the project every command reads when no flag, variable or
// board.yml selects one (section 5.3).
const DefaultFileName = "default.yml"

// userFileExt is the extension of the per-user board files.
const userFileExt = ".yml"

// Default is the recorded default project and the file it came from;
// Project is zero when none is recorded.
type Default struct {
	Project domain.ProjectRef
	Path    string
}

// IsZero reports whether no default is recorded.
func (d Default) IsZero() bool { return d.Project.IsZero() }

// defaultFile is the content of default.yml.
type defaultFile struct {
	Project domain.ProjectRef `yaml:"project"`
}

// DefaultPath is where default.yml lives under the config directory, or
// "" when the directory is unknown.
func DefaultPath(configDir string) string {
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, DefaultFileName)
}

// ReadDefault reads the recorded default. A missing file means none; a
// file that cannot be read or decoded is a usage error that names it, so
// a damaged default never falls through to another step unnoticed (F6).
func ReadDefault(configDir string) (Default, error) {
	path := DefaultPath(configDir)
	if path == "" {
		return Default{}, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // the path is built under the kit config directory
	if errors.Is(err, fs.ErrNotExist) {
		return Default{}, nil
	}
	if err != nil {
		return Default{}, fmt.Errorf("%s: %s: %w", path, oneLine(err), domain.ErrUsage)
	}
	ref, err := DecodeDefault(data)
	if err != nil {
		return Default{}, fmt.Errorf("%s: %w (fix it with gh board use owner/number or gh board use --clear)", path, err)
	}
	return Default{Project: ref, Path: path}, nil
}

// EncodeDefault renders the content of default.yml for a project.
func EncodeDefault(ref domain.ProjectRef) []byte {
	body, err := yaml.Marshal(defaultFile{Project: ref})
	if err != nil {
		// A struct of two scalars always marshals; the fallback keeps the
		// signature free of an error nobody can act on.
		body = []byte(fmt.Sprintf("project:\n    owner: %q\n    number: %d\n", ref.Owner, ref.Number))
	}
	header := "# Default project of gh board, written by gh board use.\n# Change it with gh board use owner/number, forget it with gh board use --clear.\n"
	return append([]byte(header), body...)
}

// DecodeDefault parses default.yml strictly: only the project key, with
// an owner and a positive number.
func DecodeDefault(data []byte) (domain.ProjectRef, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var file defaultFile
	if err := dec.Decode(&file); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.ProjectRef{}, fmt.Errorf("default.yml is empty: %w", domain.ErrUsage)
		}
		return domain.ProjectRef{}, fmt.Errorf("default.yml: %s: %w", oneLine(err), domain.ErrUsage)
	}
	ref, err := domain.ParseProjectRef(file.Project.String())
	if err != nil || ref != file.Project {
		return domain.ProjectRef{}, fmt.Errorf("default.yml: project needs an owner and a positive number: %w", domain.ErrUsage)
	}
	return ref, nil
}

// UserFiles lists the per-user board files under the config directory in
// name order: every regular .yml file except default.yml. A directory
// that cannot be read lists nothing, like a directory that is not there.
func UserFiles(configDir string) []string {
	if configDir == "" {
		return nil
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(name, userFileExt) && name != DefaultFileName {
			out = append(out, filepath.Join(configDir, name))
		}
	}
	return out
}

// ProjectFromFileName reads the project a per-user file is named after,
// <owner>-<number>.yml. An owner may hold dashes, so the number is what
// follows the last one.
func ProjectFromFileName(path string) (domain.ProjectRef, bool) {
	name := strings.TrimSuffix(filepath.Base(path), userFileExt)
	i := strings.LastIndex(name, "-")
	if i <= 0 {
		return domain.ProjectRef{}, false
	}
	n, err := strconv.Atoi(name[i+1:])
	if err != nil || n <= 0 {
		return domain.ProjectRef{}, false
	}
	return domain.ProjectRef{Owner: name[:i], Number: n}, true
}
