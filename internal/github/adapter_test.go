package github_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/github"
)

func TestNew(t *testing.T) {
	a := github.New(github.Options{Host: "github.com", CacheDir: "/tmp/x"})
	if a == nil || a.Options().CacheTTL != github.DefaultCacheTTL || a.Options().Host != "github.com" {
		t.Fatalf("Options = %+v", a.Options())
	}
	custom := github.New(github.Options{CacheTTL: time.Minute})
	if custom.Options().CacheTTL != time.Minute {
		t.Fatal("explicit TTL must be kept")
	}
	if now := (github.SystemClock{}).Now(); time.Since(now) > time.Minute {
		t.Fatal("SystemClock is off")
	}
	if a.Options().Clock != nil || a.Options().Transport != nil {
		t.Fatal("defaults are applied without rewriting the options")
	}
}
