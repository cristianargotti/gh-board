package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/cristianargotti/gh-board/internal/render"
)

// readDevVersion is the version of a build without ldflags.
const readDevVersion = "dev"

// readExecutable resolves the running binary; tests point it at a fixture.
var readExecutable = os.Executable

// readReleaseChecksummer is the narrow port doctor needs to compare the
// binary with the release checksum (section 14).
type readReleaseChecksummer interface {
	ReleaseChecksum(ctx context.Context, version, asset string) (string, error)
}

// readReleaseDownloader is the port doctor uses to fetch the release asset
// when the installed file differs, so that the published asset itself is
// checked against the manifest and a re-signed copy is compared with it.
type readReleaseDownloader interface {
	DownloadReleaseAsset(ctx context.Context, version, asset string, w io.Writer) error
}

// Provenance findings about the release asset.
const (
	readProvenanceMatches = "release asset matches the manifest"
	readProvenanceDiffers = "release asset differs from the manifest"
	readProvenanceMissing = "release asset not downloaded: "
)

// readNoteResigned opens the note about a binary gh re-signed on Apple
// silicon: codesign rewrites the signature after the download, so the
// bytes differ from the asset while the code is the same.
const readNoteResigned = "installed copy re-signed by macOS after download (gh runs codesign on Apple silicon)"

// readFileSHA256 streams a file through SHA-256, so a large executable is
// never held in memory.
func readFileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // the running binary, resolved by os.Executable
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readDoctorBinaryCheck reports the binary in use and compares it with the
// release checksum when one is available; an installed file that differs
// is then compared with the release asset itself.
func readDoctorBinaryCheck(ctx context.Context, deps *Deps, rep *readDoctorReport) {
	path, err := readExecutable()
	if err != nil {
		rep.Binary.Note = readNoteBinaryPath + err.Error()
		return
	}
	rep.Binary.Path = path
	sum, err := readFileSHA256(path)
	if err != nil {
		rep.Binary.Note = "binary unreadable: " + err.Error()
		return
	}
	rep.Binary.SHA256 = sum
	release, note := readReleaseChecksum(ctx, deps)
	rep.Binary.ReleaseChecksum, rep.Binary.Note = release, note
	if release == "" {
		return
	}
	if release == sum {
		rep.Binary.Verified = readBoolPtr(true)
		return
	}
	readDoctorBinaryDiffers(ctx, deps, rep, path)
}

func readReleaseChecksum(ctx context.Context, deps *Deps) (string, string) {
	if deps.Version == "" || deps.Version == readDevVersion {
		return "", "development build: no release checksum to compare"
	}
	r, ok := deps.Reader.(readReleaseChecksummer)
	if !ok {
		return "", "release checksum unavailable: the adapter does not expose release checksums"
	}
	sum, err := r.ReleaseChecksum(ctx, deps.Version, readReleaseAsset(deps.Version))
	if err != nil {
		return "", "release checksum unavailable: " + err.Error()
	}
	return sum, ""
}

// readDoctorBinaryDiffers handles an installed file whose hash is not the
// release checksum. The release asset is downloaded once and checked
// against the manifest; on Apple silicon gh re-signs the binary after the
// download, so a re-signed copy is compared with the asset apart from its
// code signature instead of being reported as tampered.
func readDoctorBinaryDiffers(ctx context.Context, deps *Deps, rep *readDoctorReport, path string) {
	signature := readMachOSignatureOf(path)
	rep.Binary.Signature = signature.String()
	asset := readReleaseAsset(deps.Version)
	temp, sum, err := readDownloadAsset(ctx, deps, asset)
	if temp != "" {
		defer func() { _ = os.Remove(temp) }()
	}
	rep.Binary.AssetSHA256 = sum
	switch {
	case err != nil:
		rep.Binary.Provenance = readProvenanceMissing + err.Error()
	case sum == rep.Binary.ReleaseChecksum:
		rep.Binary.Provenance = readProvenanceMatches
	default:
		rep.Binary.Provenance = readProvenanceDiffers
		rep.problem("binary: " + readProvenanceDiffers)
	}
	if !signature.Resigned() {
		rep.Binary.Verified = readBoolPtr(false)
		rep.problem("binary: checksum differs from the release checksum")
		return
	}
	readDoctorResigned(deps, rep, path, temp, asset)
}

// readDoctorResigned compares a copy that gh re-signed with the release
// asset and points to the verifications a person can run on the asset.
func readDoctorResigned(deps *Deps, rep *readDoctorReport, path, temp, asset string) {
	pointers := fmt.Sprintf("; verify the download with gh attestation verify %s --repo %s and compare its SHA-256 with checksums.txt",
		asset, deps.ReleaseRepository)
	if rep.Binary.Provenance != readProvenanceMatches {
		rep.Binary.Note = readNoteResigned + ": not compared with the release asset" + pointers
		if rep.Binary.Provenance == readProvenanceDiffers {
			rep.Binary.Verified = readBoolPtr(false)
		}
		return
	}
	same, err := readMachOEqualExceptSignature(path, temp)
	if err != nil {
		rep.Binary.Note = readNoteResigned + ": not compared with the release asset (" + err.Error() + ")" + pointers
		return
	}
	rep.Binary.Verified = readBoolPtr(same)
	if !same {
		rep.problem("binary: installed copy differs from the release asset beyond the code signature")
		return
	}
	rep.Binary.Note = readNoteResigned + ": it matches the release asset apart from the code signature" + pointers
}

// readDownloadAsset fetches the release asset into a temporary file and
// returns its path and SHA-256; the caller removes the file.
func readDownloadAsset(ctx context.Context, deps *Deps, asset string) (string, string, error) {
	d, ok := deps.Reader.(readReleaseDownloader)
	if !ok {
		return "", "", errors.New("the adapter does not download release assets")
	}
	f, err := os.CreateTemp("", "gh-board-asset-*")
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	err = d.DownloadReleaseAsset(ctx, deps.Version, asset, io.MultiWriter(f, h))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", "", err
	}
	return f.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

// readReleaseAsset names the release asset of this platform the way gh
// extension expects it (section 11).
func readReleaseAsset(version string) string {
	name := fmt.Sprintf("gh-board_%s_%s-%s", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func readBoolPtr(b bool) *bool { return &b }

// readDoctorBinarySection writes the binary check; the signature, asset
// and verdict lines appear only when doctor could establish them.
func readDoctorBinarySection(sec *render.Section, b readDoctorBinary) {
	sec.AddKeyValue("Path", b.Path).AddKeyValue("SHA-256", b.SHA256).AddKeyValue("Release checksum", b.ReleaseChecksum)
	if b.Signature != "" {
		sec.AddKeyValue("Signature", b.Signature)
	}
	if b.AssetSHA256 != "" {
		sec.AddKeyValue("Asset SHA-256", b.AssetSHA256)
	}
	if b.Provenance != "" {
		sec.AddKeyValue("Provenance", b.Provenance)
	}
	if b.Verified != nil {
		sec.AddKeyValue("Verified", readYesNo(*b.Verified))
	}
	readDoctorNote(sec, b.Note)
}
