package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func planInitPaths(local planInitLocal) error {
	paths := []string{local.Output}
	if local.Forms {
		for _, name := range planFormNames {
			paths = append(paths, filepath.Join(filepath.Dir(local.Output), ".github", "ISSUE_TEMPLATE", name+".yml"))
		}
	}
	for _, path := range paths {
		_, err := os.Lstat(path)
		if err == nil {
			return domain.Errorf(domain.ExitUsage, "local output already exists: %s", render.Title(path))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func planInitWrite(local planInitLocal, project domain.Project) error {
	if local.Config == nil {
		return domain.Errorf(domain.ExitUsage, "init plan has no board configuration")
	}
	project.Title = render.Sanitize(project.Title, 0)
	draft, err := planInitDraft(local, project)
	if err != nil {
		return err
	}
	if err := planLocalWrite(local.Output, draft); err != nil {
		return err
	}
	if !local.Forms {
		return nil
	}
	forms, err := planForms(local.Config, project, local.IssueTypes)
	if err != nil {
		return err
	}
	for _, name := range planFormNames {
		path := filepath.Join(filepath.Dir(local.Output), ".github", "ISSUE_TEMPLATE", name+".yml")
		if err := planLocalWrite(path, forms[name]); err != nil {
			return err
		}
	}
	return nil
}

func planLocalWrite(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return domain.Errorf(domain.ExitUsage, "refusing non-regular local output %s", render.Title(path))
		}
		actual, err := config.Hash(path)
		if err != nil {
			return err
		}
		want := sha256.Sum256(data)
		if actual == hex.EncodeToString(want[:]) {
			return nil
		}
		return domain.Errorf(domain.ExitDrift, "local output changed: %s", render.Title(path))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return planLocalCreate(path, data)
}

func planLocalCreate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), audit.DirPerm); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gh-board-init-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() { _ = os.Remove(name) }()
	if err := file.Close(); err != nil {
		return err
	}
	if err := audit.WriteAtomic(name, data); err != nil {
		return err
	}
	// A hard link publishes atomically without replacing an existing user file.
	return os.Link(name, path)
}

func planInitDraft(local planInitLocal, project domain.Project) ([]byte, error) {
	if len(local.IssueTypes) == 0 {
		return config.Draft(project, local.Config.Repository)
	}
	return config.DraftFrom(config.DraftInput{Project: project, Repository: local.Config.Repository, IssueTypes: local.IssueTypes})
}
