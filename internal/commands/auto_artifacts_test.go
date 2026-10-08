package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func autoFileArtifact(dir, name string, content string) autoArtifact {
	return autoArtifact{Kind: autoKindSkill, Type: autoTypeFile, Path: filepath.Join(dir, name), Content: []byte(content)}
}

func TestAutoWorkspaceFileLifecycle(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	ws := autoNewWorkspace(out, false)
	a := autoFileArtifact(dir, "own/f.txt", "one\n")
	a.OwnDir = true
	steps := []struct {
		content string
		action  string
	}{{"one\n", autoActionCreated}, {"one\n", autoActionUnchanged}, {"two\n", autoActionUpdated}}
	var last autoResult
	for i, s := range steps {
		a.Content = []byte(s.content)
		res, err := ws.install(a)
		if err != nil || res.Action != s.action || res.Entry.Created != (i == 0) {
			t.Fatalf("install %q: %+v, %v", s.content, res, err)
		}
		last = res
	}
	if data, _ := os.ReadFile(a.Path); string(data) != "two\n" {
		t.Fatalf("file = %q", data)
	}
	res, err := ws.uninstall(last.Entry)
	if err != nil || res.Action != autoActionRemoved || autoExists(filepath.Dir(a.Path)) {
		t.Fatalf("uninstall: %+v, %v", res, err)
	}
	if res, err := ws.uninstall(last.Entry); err != nil || res.Action != autoActionAbsent {
		t.Fatalf("second uninstall: %+v, %v", res, err)
	}
	if !strings.Contains(out.String(), "-two") {
		t.Fatalf("removal diff missing:\n%s", out.String())
	}
}

func TestAutoWorkspaceJSONLifecycle(t *testing.T) {
	dir := t.TempDir()
	ws := autoNewWorkspace(&bytes.Buffer{}, false)
	path := filepath.Join(dir, "hooks.json")
	foreign := map[string]any{"version": 1.0, "hooks": map[string]any{"e": []any{"theirs"}}}
	autoWriteTestFile(t, path, autoMustEncode(t, foreign))
	a := autoArtifact{Kind: autoKindGuard, Type: autoTypeJSON, Path: path, Fragment: map[string]any{
		"version": 2.0, "hooks": map[string]any{"e": []any{"ours"}, "f": []any{"x"}},
	}}
	res, err := ws.install(a)
	if err != nil || res.Action != autoActionUpdated || res.Entry.Created || len(res.Notes) != 1 {
		t.Fatalf("install: %+v, %v", res, err)
	}
	wantDelta := map[string]any{"hooks": map[string]any{"e": []any{"ours"}, "f": []any{"x"}}}
	if !reflect.DeepEqual(res.Entry.Delta, wantDelta) {
		t.Fatalf("delta = %v", res.Entry.Delta)
	}
	if res, err := ws.uninstall(res.Entry); err != nil || res.Action != autoActionUpdated {
		t.Fatalf("uninstall: %+v, %v", res, err)
	}
	if got := autoReadTestJSON(t, path); !reflect.DeepEqual(got, foreign) {
		t.Fatalf("after uninstall = %v", got)
	}
	fresh := autoArtifact{Kind: autoKindGuard, Type: autoTypeJSON, Path: filepath.Join(dir, "new.json"), Fragment: map[string]any{"a": 1.0}}
	res, err = ws.install(fresh)
	if err != nil || res.Action != autoActionCreated || !res.Entry.Created {
		t.Fatalf("create: %+v, %v", res, err)
	}
	if res, err := ws.uninstall(res.Entry); err != nil || res.Action != autoActionRemoved || autoExists(fresh.Path) {
		t.Fatalf("remove emptied file: %+v, %v", res, err)
	}
}

func autoMustEncode(t *testing.T, obj map[string]any) []byte {
	t.Helper()
	data, err := autoJSONEncode(obj)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAutoWorkspaceBlockLifecycle(t *testing.T) {
	dir := t.TempDir()
	ws := autoNewWorkspace(&bytes.Buffer{}, false)
	path := filepath.Join(dir, "AGENTS.md")
	a := autoArtifact{Kind: autoKindSkill, Type: autoTypeBlock, Path: path, Content: []byte("body\n")}
	res, err := ws.install(a)
	if err != nil || res.Action != autoActionCreated {
		t.Fatalf("install: %+v, %v", res, err)
	}
	if res, err := ws.uninstall(res.Entry); err != nil || res.Action != autoActionRemoved || autoExists(path) {
		t.Fatalf("uninstall of a created file: %+v, %v", res, err)
	}
	autoWriteTestFile(t, path, []byte("# Team\n"))
	ws = autoNewWorkspace(&bytes.Buffer{}, false)
	res, err = ws.install(a)
	if err != nil || res.Action != autoActionUpdated || res.Entry.Created {
		t.Fatalf("append: %+v, %v", res, err)
	}
	if res, err := ws.uninstall(res.Entry); err != nil || res.Action != autoActionUpdated {
		t.Fatalf("remove block: %+v, %v", res, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "# Team\n" {
		t.Fatalf("after = %q", data)
	}
	if res, err := ws.uninstall(res.Entry); err != nil || res.Action != autoActionKept {
		t.Fatalf("nothing left to remove: %+v, %v", res, err)
	}
}

func TestAutoWorkspaceDryRunOverlay(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	ws := autoNewWorkspace(out, true)
	path := filepath.Join(dir, "settings.json")
	first := autoArtifact{Type: autoTypeJSON, Path: path, Fragment: map[string]any{"a": 1.0}}
	second := autoArtifact{Type: autoTypeJSON, Path: path, Fragment: map[string]any{"a": 1.0, "b": 2.0}}
	if _, err := ws.install(first); err != nil {
		t.Fatal(err)
	}
	res, err := ws.install(second)
	if err != nil || !reflect.DeepEqual(res.Entry.Delta, map[string]any{"b": 2.0}) {
		t.Fatalf("second: %+v, %v", res, err)
	}
	if autoExists(path) || strings.Count(out.String(), "+++ ") != 2 {
		t.Fatalf("dry run wrote the file or printed %d diffs", strings.Count(out.String(), "+++ "))
	}
	if res, err := ws.uninstall(res.Entry); err != nil || res.Action != autoActionUpdated || autoExists(path) {
		t.Fatalf("dry uninstall: %+v, %v", res, err)
	}
	if _, err := ws.install(autoArtifact{Type: "weird", Path: path}); err == nil {
		t.Fatal("unknown type must fail")
	}
	if _, _, err := autoRevertArtifact(autoReceiptEntry{Type: "weird"}, nil); err == nil {
		t.Fatal("unknown type must fail")
	}
}

func TestAutoWorkspaceErrors(t *testing.T) {
	dir := t.TempDir()
	ws := autoNewWorkspace(&bytes.Buffer{}, false)
	if _, err := ws.install(autoFileArtifact(dir, "", "x")); err == nil {
		t.Fatal("a directory as path must fail")
	}
	if _, err := ws.uninstall(autoReceiptEntry{Type: autoTypeFile, Path: dir}); err == nil {
		t.Fatal("a directory as path must fail")
	}
	bad := filepath.Join(dir, "bad.json")
	autoWriteTestFile(t, bad, []byte("[1]"))
	if _, err := ws.uninstall(autoReceiptEntry{Type: autoTypeJSON, Path: bad, Delta: map[string]any{"a": 1.0}}); err == nil {
		t.Fatal("damaged json on uninstall must fail")
	}
	if _, err := ws.install(autoArtifact{Type: autoTypeJSON, Path: bad, Fragment: map[string]any{"a": 1.0}}); err == nil {
		t.Fatal("damaged json on install must fail")
	}
}

func TestAutoWriteFileKeepsMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("a\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := autoWriteFile(path, []byte("b\n"), true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
	fresh := filepath.Join(filepath.Dir(path), "new", "f")
	if err := autoWriteFile(fresh, []byte("c"), false); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(fresh); info.Mode().Perm() != 0o600 {
		t.Fatalf("new file mode = %v", info.Mode())
	}
}

func TestAutoAsset(t *testing.T) {
	if data, err := autoAsset(autoAssetSkill, ""); err != nil || len(data) == 0 {
		t.Fatalf("skill: %d bytes, %v", len(data), err)
	}
	if data, err := autoAsset("agents/nope.md", "fallback"); err != nil || string(data) != "fallback" {
		t.Fatalf("fallback: %q, %v", data, err)
	}
	if _, err := autoAsset("agents/nope.md", ""); err == nil {
		t.Fatal("a missing asset without fallback must fail")
	}
}
