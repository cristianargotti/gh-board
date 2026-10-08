package alerts_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

const exampleFile = "testdata/alerts.example.json"

func TestStateRoundTrip(t *testing.T) {
	path := alerts.StatePath(filepath.Join(t.TempDir(), "state", "nested"))
	want := stateOf(fxProject, late, wip)
	if err := alerts.WriteState(path, want); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	assertPerm(t, path)
	got, err := alerts.ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if !bytes.Equal(wantJSON, gotJSON) {
		t.Fatalf("state = %s\nwant    %s", gotJSON, wantJSON)
	}
}

func TestWriteStateNeverNullAlerts(t *testing.T) {
	path := alerts.StatePath(t.TempDir())
	if err := alerts.WriteState(path, domain.AlertState{Project: fxProject}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"alerts": []`) || !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("content = %q, %v", data, err)
	}
	if err := alerts.WriteState(t.TempDir(), domain.AlertState{}); err == nil {
		t.Fatal("writing over a directory must fail")
	}
}

func TestExampleDocumentsTheShape(t *testing.T) {
	state, err := alerts.ReadState(exampleFile)
	if err != nil {
		t.Fatalf("ReadState(%s): %v", exampleFile, err)
	}
	if len(state.Alerts) != 3 || state.Project != fxProject || state.Alerts[2].Item.NodeID != "" {
		t.Fatalf("example decoded as %+v", state)
	}
	path := alerts.StatePath(t.TempDir())
	if err := alerts.WriteState(path, state); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	original, _ := os.ReadFile(exampleFile)
	written, _ := os.ReadFile(path)
	if !bytes.Equal(bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n")), written) {
		t.Fatalf("the example is not what WriteState writes:\n%s", written)
	}
}

var readStateCases = []struct {
	name    string
	content string
	missing bool
	dir     bool
	err     string
}{
	{name: "a missing file is an empty state", missing: true},
	{name: "a corrupt file is an error", content: "{not json", err: "decode"},
	{name: "a directory is an error", dir: true, err: "read"},
	{name: "an empty object decodes", content: "{}"},
}

// statePathOf lays out the file of a read case and returns the path to read.
func statePathOf(t *testing.T, dir, content string, missing, isDir bool) string {
	t.Helper()
	if isDir {
		return dir
	}
	path := alerts.StatePath(dir)
	if !missing {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestReadState(t *testing.T) {
	for _, tc := range readStateCases {
		t.Run(tc.name, func(t *testing.T) {
			path := statePathOf(t, t.TempDir(), tc.content, tc.missing, tc.dir)
			state, err := alerts.ReadState(path)
			if wantError(t, err, tc.err) {
				return
			}
			if len(state.Alerts) != 0 {
				t.Fatalf("state = %+v", state)
			}
		})
	}
}

func TestPaths(t *testing.T) {
	if got := alerts.StatePath("s"); got != filepath.Join("s", "alerts.json") {
		t.Fatalf("StatePath = %s", got)
	}
	if got := alerts.LockPath("s"); got != filepath.Join("s", "watch.lock") {
		t.Fatalf("LockPath = %s", got)
	}
}
