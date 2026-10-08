package github

import (
	"context"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// releaseDigestPrefix is how GitHub spells the digest of a release asset.
const releaseDigestPrefix = "sha256:"

// ReleaseChecksum returns the SHA-256 hex digest of a release asset of
// the kit's own repository (Options.ReleaseRepository), which doctor
// compares with the installed binary. The tag is the version with the v
// prefix of the release tags.
func (a *Adapter) ReleaseChecksum(ctx context.Context, version, asset string) (string, error) {
	if a.opts.ReleaseRepository == "" {
		return "", usage("release_assets: no release repository is configured")
	}
	owner, name, err := domain.SplitRepository(a.opts.ReleaseRepository)
	if err != nil {
		return "", err
	}
	if version == "" || asset == "" {
		return "", usage("release_assets: a version and an asset name are required")
	}
	tag := releaseTag(version)
	var out struct {
		Repository *struct {
			Release *struct {
				TagName       string `json:"tagName"`
				ReleaseAssets struct {
					Nodes []struct {
						Name   string `json:"name"`
						Digest string `json:"digest"`
					} `json:"nodes"`
				} `json:"releaseAssets"`
			} `json:"release"`
		} `json:"repository"`
	}
	if err := a.run(ctx, "release_assets", map[string]any{varOwner: owner, varName: name, varTag: tag}, &out); err != nil {
		return "", err
	}
	if out.Repository == nil || out.Repository.Release == nil {
		return "", notFound("release_assets", "release "+tag+" of "+a.opts.ReleaseRepository)
	}
	for _, node := range out.Repository.Release.ReleaseAssets.Nodes {
		if node.Name == asset {
			return assetDigest(node.Digest)
		}
	}
	return "", notFound("release_assets", "asset "+asset+" in release "+tag)
}

// releaseTag names the release tag of a version: goreleaser tags v1.2.3
// and sets the version without the prefix.
func releaseTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// assetDigest keeps the hex part of a sha256 digest; any other algorithm
// is an API surprise the kit refuses to compare.
func assetDigest(digest string) (string, error) {
	if !strings.HasPrefix(digest, releaseDigestPrefix) {
		return "", &APIError{Operation: "release_assets", Message: "asset digest is not sha256: " + digest, kind: domain.ErrAPI}
	}
	return strings.TrimPrefix(digest, releaseDigestPrefix), nil
}
