package plan

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// idPattern keeps plan ids usable as file names on every OS.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// validID reports whether id is safe as a file name.
func validID(id string) bool {
	return idPattern.MatchString(id) && !strings.Contains(id, "..")
}

// NewID returns a plan id that sorts by creation time: the domain layout
// seeded with random bytes, so two plans of the same second differ.
func NewID(now time.Time) (string, error) {
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", fmt.Errorf("plan id: %w", err)
	}
	return domain.NewPlanID(now, hex.EncodeToString(seed[:])), nil
}

// Path returns where a plan id lives under the state directory.
func Path(dir, id string) string {
	return filepath.Join(dir, PlansDir, id+".json")
}

// Write stores the plan as <dir>/plans/<id>.json with owner-only
// permissions: it fills the expiry when empty, numbers the steps in order
// and stores the hash of the canonical content. It returns the path. A
// plan is immutable, so writing an id that exists is refused.
func Write(dir string, p domain.Plan) (string, error) {
	if !validID(p.ID) {
		return "", fmt.Errorf("plan id %q is not valid: %w", p.ID, domain.ErrUsage)
	}
	if p.CreatedAt.IsZero() {
		return "", fmt.Errorf("plan %s has no creation time: %w", p.ID, domain.ErrUsage)
	}
	if p.ExpiresAt.IsZero() {
		p.ExpiresAt = p.CreatedAt.Add(domain.PlanExpiry)
	}
	p.Steps = append([]domain.Step(nil), p.Steps...)
	for i := range p.Steps {
		p.Steps[i].Index = i
	}
	hash, err := p.ComputeHash()
	if err != nil {
		return "", fmt.Errorf("hash plan %s: %w", p.ID, err)
	}
	p.Hash = hash
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode plan %s: %w", p.ID, err)
	}
	path := Path(dir, p.ID)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("plan %s already exists at %s: %w", p.ID, path, domain.ErrUsage)
	}
	if err := audit.WriteAtomic(path, append(data, '\n')); err != nil {
		return "", err
	}
	return path, nil
}

// Read loads a plan by id or by path and verifies its hash. A missing
// plan is ErrNotFound; an edited one is ErrApplyRefused.
func Read(dir, ref string) (domain.Plan, error) {
	path, err := resolve(dir, ref)
	if err != nil {
		return domain.Plan{}, err
	}
	p, err := decode(path)
	if err != nil {
		return domain.Plan{}, err
	}
	if err := p.Verify(); err != nil {
		return domain.Plan{}, err
	}
	return p, nil
}

// resolve turns an id or a path into the file to read.
func resolve(dir, ref string) (string, error) {
	if validID(ref) {
		if path := Path(dir, ref); exists(path) {
			return path, nil
		}
	}
	if ref != "" && exists(ref) {
		return ref, nil
	}
	return "", fmt.Errorf("plan %q: %w", ref, domain.ErrNotFound)
}

// exists reports whether path is a regular file.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// decode reads one plan file.
func decode(path string) (domain.Plan, error) {
	data, err := os.ReadFile(path) //nolint:gosec // plan file chosen by the user
	if err != nil {
		return domain.Plan{}, fmt.Errorf("read plan: %w", err)
	}
	var p domain.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return domain.Plan{}, fmt.Errorf("decode plan %s: %w", path, err)
	}
	return p, nil
}

// List returns the plans in the state directory, newest first. Files that
// do not decode are skipped; an edited plan is listed and refused later.
// A state directory never written has no plans; a plans path that is a
// file, or runs through one, is an error on every operating system.
func List(dir string) ([]domain.Plan, error) {
	entries, err := audit.ReadDir(filepath.Join(dir, PlansDir))
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	var plans []domain.Plan
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p, err := decode(filepath.Join(dir, PlansDir, e.Name()))
		if err != nil {
			continue
		}
		plans = append(plans, p)
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if !plans[i].CreatedAt.Equal(plans[j].CreatedAt) {
			return plans[i].CreatedAt.After(plans[j].CreatedAt)
		}
		return plans[i].ID > plans[j].ID
	})
	return plans, nil
}

// Prune removes plan and journal files created before the instant and
// returns how many files went away (retention of section 6.9). A plan
// file that no longer decodes is judged by its modification time, so a
// damaged file does not escape retention.
func Prune(dir string, before time.Time) (int, error) {
	entries, err := audit.ReadDir(filepath.Join(dir, PlansDir))
	if err != nil {
		return 0, fmt.Errorf("list plans: %w", err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if !createdBefore(Path(dir, id), e, before) {
			continue
		}
		for _, path := range []string{Path(dir, id), journalPath(dir, id)} {
			if err := os.Remove(path); err == nil {
				removed++
			} else if !errors.Is(err, os.ErrNotExist) {
				return removed, fmt.Errorf("prune %s: %w", path, err)
			}
		}
	}
	return removed, nil
}

// createdBefore reads the creation time of a plan file, falling back to
// the file time when the content does not decode.
func createdBefore(path string, e os.DirEntry, before time.Time) bool {
	if p, err := decode(path); err == nil {
		return p.CreatedAt.Before(before)
	}
	info, err := e.Info()
	return err == nil && info.ModTime().Before(before)
}
