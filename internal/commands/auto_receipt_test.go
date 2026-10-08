package commands

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/gh-board/internal/config"
	"github.com/cristianargotti/gh-board/internal/guard"
)

func TestAutoReceiptRoundTrip(t *testing.T) {
	dirs := config.Paths(t.TempDir())
	path := autoReceiptPath(dirs, guard.AgentCursor, autoScopeProject)
	if filepath.Base(path) != "cursor-project.json" {
		t.Fatalf("path = %s", path)
	}
	if _, found, err := autoReadReceipt(path); found || err != nil {
		t.Fatalf("missing receipt: found %v, err %v", found, err)
	}
	r := autoReceipt{Agent: guard.AgentCursor, Scope: autoScopeProject, Entries: []autoReceiptEntry{
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: "/h/.cursor/hooks.json", Delta: map[string]any{"a": 1.0}, SHA256: "x"},
	}}
	if err := autoWriteReceipt(path, r); err != nil {
		t.Fatal(err)
	}
	got, found, err := autoReadReceipt(path)
	if err != nil || !found || !reflect.DeepEqual(got, r) {
		t.Fatalf("read = %+v, found %v, err %v", got, found, err)
	}
	if err := autoDeleteReceipt(path); err != nil || autoExists(path) {
		t.Fatalf("delete: %v", err)
	}
	if err := autoDeleteReceipt(path); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	autoWriteTestFile(t, path, []byte("{broken"))
	if _, _, err := autoReadReceipt(path); err == nil {
		t.Fatal("a damaged receipt must fail")
	}
}

func TestAutoReceiptUpsertAndSplit(t *testing.T) {
	var r autoReceipt
	r.upsert(autoReceiptEntry{Kind: autoKindGuard, Type: autoTypeJSON, Path: "s.json", Created: true, Delta: map[string]any{"a": []any{"x"}}})
	r.upsert(autoReceiptEntry{Kind: autoKindSkill, Type: autoTypeFile, Path: "SKILL.md", Created: true})
	r.upsert(autoReceiptEntry{Kind: autoKindGuard, Type: autoTypeJSON, Path: "s.json", Created: false, Delta: map[string]any{"a": []any{"y"}}})
	if len(r.Entries) != 2 {
		t.Fatalf("entries = %+v", r.Entries)
	}
	merged := r.Entries[0]
	if !merged.Created || !reflect.DeepEqual(merged.Delta, map[string]any{"a": []any{"x", "y"}}) {
		t.Fatalf("merged = %+v", merged)
	}
	chosen, rest := r.split([]autoKind{autoKindGuard})
	if len(chosen) != 1 || len(rest) != 1 || chosen[0].Path != "s.json" || rest[0].Path != "SKILL.md" {
		t.Fatalf("split = %v / %v", chosen, rest)
	}
	if autoKindIn(nil, autoKindGuard) {
		t.Fatal("empty kinds contain nothing")
	}
}

func TestAutoInstalledArtifacts(t *testing.T) {
	dirs := config.Paths(t.TempDir())
	if got, err := autoInstalledArtifacts(dirs); err != nil || len(got) != 0 {
		t.Fatalf("no receipts: %v, %v", got, err)
	}
	one := autoReceipt{Agent: guard.AgentClaude, Scope: autoScopeUser, Entries: []autoReceiptEntry{
		{Kind: autoKindGuard, Type: autoTypeJSON, Path: "/h/.claude/settings.json", SHA256: "merged"},
	}}
	two := autoReceipt{Agent: guard.AgentCodex, Scope: autoScopeUser, Entries: []autoReceiptEntry{
		{Kind: autoKindGuard, Type: autoTypeFile, Path: "/h/.codex/rules/gh-board.rules", SHA256: "whole"},
	}}
	for _, r := range []autoReceipt{one, two} {
		if err := autoWriteReceipt(autoReceiptPath(dirs, r.Agent, r.Scope), r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := autoInstalledArtifacts(dirs)
	want := []guard.Artifact{{Path: "/h/.codex/rules/gh-board.rules", SHA256: "whole"}, {Path: "/h/.claude/settings.json", SHA256: "merged"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts = %v, %v", got, err)
	}
	autoWriteTestFile(t, autoReceiptPath(dirs, guard.AgentCursor, autoScopeUser), []byte("{broken"))
	if _, err := autoInstalledArtifacts(dirs); err == nil {
		t.Fatal("a damaged receipt must fail")
	}
}
