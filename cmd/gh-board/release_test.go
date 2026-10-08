package main

import (
	"runtime/debug"
	"testing"
)

var repositoryCases = []struct {
	path, want string
}{
	{"github.com/cristianargotti/gh-board", "cristianargotti/gh-board"},
	{"github.com/acme/gh-board/v2", "acme/gh-board"},
	{"gitlab.com/acme/gh-board", ""},
	{"github.com/acme", ""},
	{"github.com//x", ""},
	{"", ""},
}

func TestRepositoryOf(t *testing.T) {
	for _, tc := range repositoryCases {
		if got := repositoryOf(tc.path); got != tc.want {
			t.Errorf("repositoryOf(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestReleaseRepository(t *testing.T) {
	original := readBuildInfo
	t.Cleanup(func() { readBuildInfo = original })
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/acme/gh-board"}}, true
	}
	if got := releaseRepository(); got != "acme/gh-board" {
		t.Fatalf("releaseRepository() = %q", got)
	}
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	if got := releaseRepository(); got != "" {
		t.Fatalf("without build info = %q", got)
	}
}
