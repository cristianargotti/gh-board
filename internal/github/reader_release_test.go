package github_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/github"
)

// The digest GitHub reported for the recorded asset of cli/cli v2.102.0.
const recordedAssetDigest = "7e54a307f90afdc597"

var releaseCases = []struct {
	name, repository, version, asset string
	err                              error
}{
	{"version without prefix", "cli/cli", "2.102.0", "gh_2.102.0_linux_amd64.deb", nil},
	{"version with prefix", "cli/cli", "v2.102.0", "gh_2.102.0_linux_amd64.deb", nil},
	{"asset missing", "cli/cli", "2.102.0", "gh-board_2.102.0_linux-amd64", domain.ErrNotFound},
	{"release missing", "cli/cli", "0.0.0-not-a-release", "gh_2.102.0_linux_amd64.deb", domain.ErrNotFound},
	{"no repository configured", "", "2.102.0", "gh_2.102.0_linux_amd64.deb", domain.ErrUsage},
	{"malformed repository", "cli", "2.102.0", "gh_2.102.0_linux_amd64.deb", domain.ErrUsage},
	{"empty version", "cli/cli", "", "gh_2.102.0_linux_amd64.deb", domain.ErrUsage},
}

func TestReleaseChecksum(t *testing.T) {
	for _, tc := range releaseCases {
		t.Run(tc.name, func(t *testing.T) {
			_, base := newReplay(t, "release")
			a := github.New(github.Options{Transport: base.Options().Transport, Clock: fixedClock{now: testNow}, ReleaseRepository: tc.repository})
			sum, err := a.ReleaseChecksum(context.Background(), tc.version, tc.asset)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if tc.err == nil && (len(sum) != 64 || sum[:len(recordedAssetDigest)] != recordedAssetDigest) {
				t.Fatalf("checksum = %q", sum)
			}
		})
	}
}

var downloadCases = []struct {
	name, repository, asset string
	err                     error
	want                    string
}{
	{"follows the redirect", "cli/cli", "gh_2.102.0_linux_amd64.deb", nil, "deb package bytes\n"},
	{"http error", "cli/cli", "gh_2.102.0_linux_386.deb", domain.ErrAPI, ""},
	{"asset missing", "cli/cli", "gh-board_2.102.0_linux-amd64", domain.ErrNotFound, ""},
	{"no repository configured", "", "gh_2.102.0_linux_amd64.deb", domain.ErrUsage, ""},
}

func TestDownloadReleaseAsset(t *testing.T) {
	for _, tc := range downloadCases {
		t.Run(tc.name, func(t *testing.T) {
			r, base := newReplay(t, "release")
			a := github.New(github.Options{Transport: base.Options().Transport, Clock: fixedClock{now: testNow}, ReleaseRepository: tc.repository})
			var buf bytes.Buffer
			err := a.DownloadReleaseAsset(context.Background(), "2.102.0", tc.asset, &buf)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if buf.String() != tc.want {
				t.Fatalf("downloaded %q, want %q", buf.String(), tc.want)
			}
			if tc.err == nil && r.count() != 3 {
				t.Fatalf("calls = %+v, want the query, the redirect and the object", r.all())
			}
		})
	}
}
