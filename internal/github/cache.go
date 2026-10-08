package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// Schema cache layout and permissions (section 6.9): the cache holds the
// board schema only, never items, under the kit cache directory.
const (
	schemaDir = "schema"
	dirPerm   = 0o700
)

// schemaEntry is the cache file. The host keys it with the project because
// the same owner and number exist on every GitHub host.
type schemaEntry struct {
	Host    string         `json:"host"`
	Project domain.Project `json:"project"`
}

func (a *Adapter) cachePath(host string, ref domain.ProjectRef) string {
	if a.opts.CacheDir == "" {
		return ""
	}
	owner := url.PathEscape(strings.ToLower(ref.Owner))
	name := fmt.Sprintf("%s_%s-%d.json", url.PathEscape(host), owner, ref.Number)
	return filepath.Join(a.opts.CacheDir, schemaDir, name)
}

// readCache returns the cached schema when it exists, belongs to the host
// and is younger than the TTL. Anything else is a miss, never an error:
// a broken cache is refetched and rewritten.
func (a *Adapter) readCache(host string, ref domain.ProjectRef) (domain.Project, bool) {
	path := a.cachePath(host, ref)
	if path == "" {
		return domain.Project{}, false
	}
	data, err := os.ReadFile(path) //nolint:gosec // the path is built under the kit cache directory
	if err != nil {
		return domain.Project{}, false
	}
	var entry schemaEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Host != host {
		return domain.Project{}, false
	}
	age := a.clock.Now().Sub(entry.Project.DiscoveredAt)
	if age < 0 || age >= a.opts.CacheTTL {
		return domain.Project{}, false
	}
	return entry.Project, true
}

// writeCache stores the schema atomically with owner-only permissions.
func (a *Adapter) writeCache(host string, p domain.Project) error {
	path := a.cachePath(host, p.Ref)
	if path == "" {
		return nil
	}
	data, err := json.Marshal(schemaEntry{Host: host, Project: p})
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// writeAtomic writes through a temporary file in the same directory and
// renames it, so a reader never sees a partial file. The temporary file is
// created with owner-only permissions.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".schema-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func writeAndClose(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
