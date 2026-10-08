package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cristianargotti/gh-board/internal/domain"
)

// releaseDigestPrefix is how GitHub spells the digest of a release asset.
const releaseDigestPrefix = "sha256:"

// maxReleaseAssetBytes bounds a release asset download; the kit binaries
// are near ten megabytes.
const maxReleaseAssetBytes = 256 << 20

// releaseAsset is one asset of the release_assets document.
type releaseAsset struct {
	Name        string `json:"name"`
	Digest      string `json:"digest"`
	DownloadURL string `json:"downloadUrl"`
}

// releaseQuery is the shape of the release_assets document.
type releaseQuery struct {
	Repository *struct {
		Release *struct {
			TagName       string `json:"tagName"`
			ReleaseAssets struct {
				Nodes []releaseAsset `json:"nodes"`
			} `json:"releaseAssets"`
		} `json:"release"`
	} `json:"repository"`
}

// ReleaseChecksum returns the SHA-256 hex digest of a release asset of
// the kit's own repository (Options.ReleaseRepository), which doctor
// compares with the installed binary. The tag is the version with the v
// prefix of the release tags.
func (a *Adapter) ReleaseChecksum(ctx context.Context, version, asset string) (string, error) {
	node, err := a.releaseAsset(ctx, version, asset)
	if err != nil {
		return "", err
	}
	return assetDigest(node.Digest)
}

// DownloadReleaseAsset streams a release asset into w through the client
// of gh, following the redirect to the object store. Doctor hashes it to
// check the published asset against the manifest when the installed copy
// differs from it, and compares a copy that gh re-signed with it.
func (a *Adapter) DownloadReleaseAsset(ctx context.Context, version, asset string, w io.Writer) error {
	node, err := a.releaseAsset(ctx, version, asset)
	if err != nil {
		return err
	}
	conn, err := a.connect()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, node.DownloadURL, nil)
	if err != nil {
		return downloadError(err.Error())
	}
	resp, err := conn.http.Do(req)
	if err != nil {
		return downloadError(err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return downloadError(fmt.Sprintf("%s: HTTP %d", asset, resp.StatusCode))
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxReleaseAssetBytes+1))
	switch {
	case err != nil:
		return downloadError(err.Error())
	case n > maxReleaseAssetBytes:
		return downloadError(fmt.Sprintf("%s exceeds %d bytes", asset, maxReleaseAssetBytes))
	}
	return nil
}

func downloadError(message string) error {
	return &APIError{Operation: "release_download", Message: message, kind: domain.ErrAPI}
}

// releaseAsset finds one asset of the release of a version.
func (a *Adapter) releaseAsset(ctx context.Context, version, asset string) (releaseAsset, error) {
	if a.opts.ReleaseRepository == "" {
		return releaseAsset{}, usage("release_assets: no release repository is configured")
	}
	owner, name, err := domain.SplitRepository(a.opts.ReleaseRepository)
	if err != nil {
		return releaseAsset{}, err
	}
	if version == "" || asset == "" {
		return releaseAsset{}, usage("release_assets: a version and an asset name are required")
	}
	tag := releaseTag(version)
	var out releaseQuery
	if err := a.run(ctx, "release_assets", map[string]any{varOwner: owner, varName: name, varTag: tag}, &out); err != nil {
		return releaseAsset{}, err
	}
	if out.Repository == nil || out.Repository.Release == nil {
		return releaseAsset{}, notFound("release_assets", "release "+tag+" of "+a.opts.ReleaseRepository)
	}
	for _, node := range out.Repository.Release.ReleaseAssets.Nodes {
		if node.Name == asset {
			return node, nil
		}
	}
	return releaseAsset{}, notFound("release_assets", "asset "+asset+" in release "+tag)
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
