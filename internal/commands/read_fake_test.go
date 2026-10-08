package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/domain"
)

type readFakeClock struct{ now time.Time }

func (c readFakeClock) Now() time.Time { return c.now }

// readFakeReader serves the fixture board. fail maps a method name to the
// error it returns; pageSize forces pagination even with All; stuck
// returns a cursor that never advances.
type readFakeReader struct {
	project    domain.Project
	items      []domain.Item
	viewer     domain.Viewer
	permission domain.Permission
	labels     []domain.Label
	milestones []domain.Milestone
	issueTypes []domain.IssueType
	comments   []domain.Comment
	pageSize   int
	stuck      bool
	fail       map[string]error
	calls      []string
	selections []domain.ItemSelection
}

func readNewFakeReader() *readFakeReader {
	later := readDate("2027-01-31")
	return &readFakeReader{
		project: readFixtureProject(), items: readFixtureItems(),
		viewer:     domain.Viewer{Login: "ana", ID: "U_kgDOana", Host: "github.com", TokenSource: "oauth_token"}, //nolint:gosec // the token source name, never a credential
		permission: domain.PermissionWrite,
		labels: []domain.Label{
			{ID: "LA_bug", Name: "bug"}, {ID: "LA_docs", Name: "docs"}, {ID: "LA_entrada", Name: "entrada"}, {ID: "LA_urgente", Name: "urgente"},
		},
		milestones: []domain.Milestone{*readFixtureMilestone(), {ID: "MI_kwDOAbc002", Number: 2, Title: "v1.1", State: "OPEN", DueOn: &later}},
		issueTypes: []domain.IssueType{{ID: "IT_Feature", Name: "Feature"}, {ID: "IT_Task", Name: "Task"}, {ID: "IT_Bug", Name: "Bug"}},
		fail:       map[string]error{},
	}
}

func (f *readFakeReader) call(method string) error {
	f.calls = append(f.calls, method)
	return f.fail[method]
}

func (f *readFakeReader) DiscoverProject(_ context.Context, ref domain.ProjectRef) (domain.Project, error) {
	if err := f.call("DiscoverProject"); err != nil {
		return domain.Project{}, err
	}
	if ref != f.project.Ref {
		return domain.Project{}, fmt.Errorf("project %s: %w", ref, domain.ErrNotFound)
	}
	return f.project, nil
}

func (f *readFakeReader) ListItems(_ context.Context, _ domain.Project, opts domain.ListOptions) (domain.ItemPage, error) {
	f.selections = append(f.selections, opts.Selection)
	if err := f.call("ListItems"); err != nil {
		return domain.ItemPage{}, err
	}
	if f.stuck {
		return domain.ItemPage{Items: f.items[:1], HasNext: true, NextCursor: "1"}, nil
	}
	start := 0
	if opts.Cursor != "" {
		start, _ = strconv.Atoi(opts.Cursor)
	}
	size := len(f.items)
	switch {
	case f.pageSize > 0:
		size = f.pageSize
	case !opts.All && opts.Limit > 0:
		size = opts.Limit
	}
	end := min(start+size, len(f.items))
	page := domain.ItemPage{Items: f.items[start:end], TotalCount: len(f.items)}
	if end < len(f.items) {
		page.HasNext, page.NextCursor = true, strconv.Itoa(end)
	}
	return page, nil
}

func (f *readFakeReader) GetItem(_ context.Context, _ domain.Project, ref domain.Reference) (domain.Item, error) {
	if err := f.call("GetItem"); err != nil {
		return domain.Item{}, err
	}
	for _, it := range f.items {
		if readRefMatches(ref, it) {
			return it, nil
		}
	}
	return domain.Item{}, fmt.Errorf("%s: %w", ref.String(), domain.ErrNotFound)
}

func readRefMatches(ref domain.Reference, it domain.Item) bool {
	switch ref.Kind {
	case domain.RefNodeID:
		return ref.NodeID == it.Issue.NodeID
	case domain.RefFull, domain.RefURL:
		return ref.Owner == it.Issue.Owner && ref.Repo == it.Issue.Repo && ref.Number == it.Issue.Number
	}
	return false
}

func (f *readFakeReader) Viewer(context.Context) (domain.Viewer, error) {
	if err := f.call("Viewer"); err != nil {
		return domain.Viewer{}, err
	}
	return f.viewer, nil
}

func (f *readFakeReader) ViewerPermission(_ context.Context, _, _ string) (domain.Permission, error) {
	if err := f.call("ViewerPermission"); err != nil {
		return "", err
	}
	return f.permission, nil
}

func (f *readFakeReader) RepositoryLabels(_ context.Context, _, _ string) ([]domain.Label, error) {
	if err := f.call("RepositoryLabels"); err != nil {
		return nil, err
	}
	return f.labels, nil
}

func (f *readFakeReader) RepositoryMilestones(_ context.Context, _, _ string) ([]domain.Milestone, error) {
	if err := f.call("RepositoryMilestones"); err != nil {
		return nil, err
	}
	return f.milestones, nil
}

func (f *readFakeReader) IssueTypes(_ context.Context, _ string) ([]domain.IssueType, error) {
	if err := f.call("IssueTypes"); err != nil {
		return nil, err
	}
	return f.issueTypes, nil
}

func (f *readFakeReader) IssueComments(_ context.Context, _ string) ([]domain.Comment, error) {
	if err := f.call("IssueComments"); err != nil {
		return nil, err
	}
	return f.comments, nil
}

// readFakeRefresher adds the narrow refresh port schema --refresh uses.
type readFakeRefresher struct{ *readFakeReader }

func (f readFakeRefresher) RefreshProject(_ context.Context, _ domain.ProjectRef) (domain.Project, error) {
	if err := f.call("RefreshProject"); err != nil {
		return domain.Project{}, err
	}
	return f.project, nil
}

// readFakeChecksums adds the narrow release checksum port doctor uses.
type readFakeChecksums struct {
	*readFakeReader
	sum   string
	err   error
	asset string
}

func (f *readFakeChecksums) ReleaseChecksum(_ context.Context, _, asset string) (string, error) {
	f.asset = asset
	return f.sum, f.err
}

// readFakeDownloads adds the release download port doctor uses when the
// installed binary differs from the release checksum.
type readFakeDownloads struct {
	*readFakeChecksums
	data []byte
	err  error
}

func (f *readFakeDownloads) DownloadReleaseAsset(_ context.Context, _, _ string, w io.Writer) error {
	if f.err != nil {
		return f.err
	}
	_, err := w.Write(f.data)
	return err
}

// readFakeTimelines adds the timeline port digest uses for first_response.
type readFakeTimelines struct {
	*readFakeReader
	timelines map[string]domain.IssueTimeline
	err       error
}

func (f readFakeTimelines) IssueTimeline(_ context.Context, id string) (domain.IssueTimeline, error) {
	if f.err != nil {
		return domain.IssueTimeline{}, f.err
	}
	return f.timelines[id], nil
}

// readTestDeps builds the dependencies of a read test with the fixture
// configuration preloaded and a temporary home.
func readTestDeps(t *testing.T, reader domain.ProjectReader) (*Deps, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	deps := &Deps{
		Reader: reader, Clock: readFakeClock{readFixtureNow}, Config: readFixtureConfig(t),
		Dirs: config.Paths(t.TempDir()), Out: out, Err: &bytes.Buffer{}, Version: "1.2.3", ReleaseRepository: "acme/gh-board",
	}
	return deps, out
}

func readRun(t *testing.T, deps *Deps, args ...string) error {
	t.Helper()
	return Execute(context.Background(), args, deps)
}

// readRunCode runs the command and returns its exit code.
func readRunCode(t *testing.T, deps *Deps, args ...string) domain.ExitCode {
	t.Helper()
	return domain.CodeOf(readRun(t, deps, args...))
}

// readFakeIssueFields adds the organization issue field port to the fake.
type readFakeIssueFields struct {
	*readFakeReader
	fields []domain.Field
	err    error
}

func (f *readFakeIssueFields) IssueFields(_ context.Context, _ string) ([]domain.Field, error) {
	return f.fields, f.err
}
