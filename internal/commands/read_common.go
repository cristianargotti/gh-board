package commands

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// Column headers and labels the read commands share, so every listing
// reads the same way.
const (
	readColRef       = "REF"
	readColTitle     = "TITLE"
	readColStatus    = "STATUS"
	readColState     = "STATE"
	readColAssignees = "ASSIGNEES"
	readColLane      = "LANE"
	readColEpic      = "EPIC"
	readColSprint    = "SPRINT"
	readColTarget    = "TARGET"
	readColStart     = "START"
	readColName      = "NAME"
	readColKind      = "KIND"
	readKeyGenerated = "Generated at"
	readKeyProject   = "Project"
	readKeyTitle     = "Title"
	readKeyName      = "Name"
	readKeyItems     = "Items"
	readKeyRole      = "Viewer role"
	readColPath      = "PATH"
	readColReason    = "REASON"
	readNoteNone     = "none"
	readUnavailable  = "unavailable: not mapped in board.yml"
	readNoReader     = "no GitHub reader configured"
	readBoardLevel   = "board"
)

// readSession is what a read command resolves before it touches the
// board: the configuration in use, the discovered project, the instant of
// the run and, once the items were read, how many board items the kit
// omits because they are not issues.
type readSession struct {
	deps    *Deps
	loaded  *config.Loaded
	cfg     *domain.Config
	project domain.Project
	now     time.Time
	skipped int
}

// readLoad resolves the configuration once per process through
// config.Load and applies the --project override on a copy, so a
// preloaded configuration is never mutated.
func readLoad(deps *Deps) (*config.Loaded, *domain.Config, error) {
	var ref domain.ProjectRef
	if deps.Flags.Project != "" {
		parsed, err := domain.ParseProjectRef(deps.Flags.Project)
		if err != nil {
			return nil, nil, err
		}
		ref = parsed
	}
	if deps.Config == nil {
		loaded, err := config.Load(config.LoadOptions{Path: deps.Flags.Config, Dirs: deps.Dirs, Project: ref})
		if err != nil {
			return nil, nil, err
		}
		deps.Config = &loaded
	}
	cfg := domain.Config{Version: config.SupportedVersion}
	if deps.Config.Config != nil {
		cfg = *deps.Config.Config
	}
	if !ref.IsZero() {
		cfg.Project = ref
	}
	return deps.Config, &cfg, nil
}

// readStart loads the configuration and discovers the project.
func readStart(ctx context.Context, deps *Deps) (*readSession, error) {
	if deps.Reader == nil {
		return nil, domain.Errorf(domain.ExitUsage, "a GitHub reader is required")
	}
	loaded, cfg, err := readLoad(deps)
	if err != nil {
		return nil, err
	}
	if cfg.Project.IsZero() {
		return nil, readNoProject(loaded)
	}
	project, err := deps.Reader.DiscoverProject(ctx, cfg.Project)
	if err != nil {
		return nil, readAPI(err)
	}
	return &readSession{deps: deps, loaded: loaded, cfg: cfg, project: project, now: deps.Now()}, nil
}

// readNoProjectText words the finding of a command that has no project:
// the ways to select one and, when the resolution found several per-user
// files, their names, so the person knows what to choose from.
func readNoProjectText(loaded *config.Loaded) string {
	if loaded == nil || len(loaded.Candidates) == 0 {
		return "no project selected: pass --project owner/number, run gh board use owner/number or configure board.yml"
	}
	names := make([]string, 0, len(loaded.Candidates))
	for _, c := range loaded.Candidates {
		names = append(names, filepath.Base(c))
	}
	return fmt.Sprintf("no project selected: %d per-user files (%s): pass --project owner/number or run gh board use owner/number",
		len(names), strings.Join(names, ", "))
}

// readNoProject is the usage error of a command that has no project.
func readNoProject(loaded *config.Loaded) error {
	return domain.Errorf(domain.ExitUsage, "%s", readNoProjectText(loaded))
}

// items reads every page of the board with everything each item holds.
func (s *readSession) items(ctx context.Context) ([]domain.Item, error) {
	return s.pages(ctx, domain.SelectionFull)
}

// summaries reads every page of the board with the summary selection:
// what the listings, the alerts and the snapshot need, without the body,
// the milestone and the parent of each issue, which makes the pages
// lighter and faster.
func (s *readSession) summaries(ctx context.Context) ([]domain.Item, error) {
	return s.pages(ctx, domain.SelectionSummary)
}

// pages reads every page of the board with the selection. The loop stops
// on a cursor that does not advance, so a misbehaving adapter cannot
// spin forever.
func (s *readSession) pages(ctx context.Context, selection domain.ItemSelection) ([]domain.Item, error) {
	var items []domain.Item
	cursor := ""
	seen := map[string]bool{}
	for {
		page, err := s.deps.Reader.ListItems(ctx, s.project, domain.ListOptions{All: true, Cursor: cursor, Selection: selection})
		if err != nil {
			return nil, readAPI(err)
		}
		items = append(items, page.Items...)
		s.skipped += page.Skipped
		if !page.HasNext {
			return items, nil
		}
		if page.NextCursor == "" || seen[page.NextCursor] {
			return nil, domain.Errorf(domain.ExitAPI, "item pagination did not advance")
		}
		cursor, seen[page.NextCursor] = page.NextCursor, true
	}
}

// generic reports whether the session runs without board.yml.
func (s *readSession) generic() bool {
	return s.loaded == nil || s.loaded.Generic()
}

// readSkippedNote states how many board items a listing left out because
// they are not issues (section 6.8), or "" when none.
func readSkippedNote(n int) string {
	if n <= 0 {
		return ""
	}
	return readCount(n, "board item", "board items") + " omitted: draft issues, pull requests or redacted items"
}

// readAPI maps a failure of the adapter to exit code 7 unless the error
// already carries a code or a sentinel (usage, not found, ambiguous).
func readAPI(err error) error {
	if err == nil {
		return nil
	}
	var coded *domain.Error
	if errors.As(err, &coded) || errors.Is(err, domain.ErrUsage) || domain.CodeOf(err) != domain.ExitUsage {
		return err
	}
	return fmt.Errorf("%w: %w", domain.ErrAPI, err)
}

// readFormat picks the renderer: --json wins over --format.
func readFormat(deps *Deps) (render.Format, error) {
	if deps.Flags.JSON {
		return render.FormatJSON, nil
	}
	if deps.Flags.Format == "" {
		return render.FormatTable, nil
	}
	return render.ParseFormat(deps.Flags.Format)
}

// readRender writes the document to the command output.
func readRender(deps *Deps, doc *render.Document) error {
	format, err := readFormat(deps)
	if err != nil {
		return err
	}
	return render.Render(deps.Out, doc, format)
}

// readStamp formats an instant for generated_at values.
func readStamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// readCount renders a count with its noun.
func readCount(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// readYesNo renders a boolean for a human.
func readYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
