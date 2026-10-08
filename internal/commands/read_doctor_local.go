package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

// readDevVersion is the version of a build without ldflags.
const readDevVersion = "dev"

// readExecutable resolves the running binary; tests point it at a fixture.
var readExecutable = os.Executable

// readReleaseChecksummer is the narrow port doctor needs to compare the
// binary with the release checksum (section 14). The adapter gains it at
// integration; until then doctor reports the checksum as unavailable.
type readReleaseChecksummer interface {
	ReleaseChecksum(ctx context.Context, version, asset string) (string, error)
}

// readDoctorArtifactsCheck verifies every file the agent installers
// recorded against the hash they left, through the guard package
// (section 6.5).
func readDoctorArtifactsCheck(deps *Deps, rep *readDoctorReport) {
	artifacts, err := autoInstalledArtifacts(deps.Dirs)
	if err != nil {
		rep.ArtifactsNote = "agent artifacts unreadable: " + err.Error()
		rep.problem(rep.ArtifactsNote)
		return
	}
	if len(artifacts) == 0 {
		rep.ArtifactsNote = "no agent artifacts recorded: install the guards with gh board agent install"
		return
	}
	results, err := guard.Verify(artifacts)
	if err != nil {
		rep.ArtifactsNote = "agent artifacts not verified: " + err.Error()
		rep.problem(rep.ArtifactsNote)
		return
	}
	for _, r := range results {
		rep.Artifacts = append(rep.Artifacts, readDoctorArtifact{Path: r.Path, OK: r.OK, Reason: r.Reason})
		if !r.OK {
			rep.problem(fmt.Sprintf("artifact %s: %s", r.Path, r.Reason))
		}
	}
}

// readFileSHA256 streams the running binary through SHA-256, so a large
// executable is never held in memory.
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
// release checksum when one is available.
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
	verified := release == sum
	rep.Binary.Verified = &verified
	if !verified {
		rep.problem("binary: checksum differs from the release checksum")
	}
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

// readReleaseAsset names the release asset of this platform the way gh
// extension expects it (section 11).
func readReleaseAsset(version string) string {
	name := fmt.Sprintf("gh-board_%s_%s-%s", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// readDoctorWatchCheck reads the alert state watch leaves for the mod.
func readDoctorWatchCheck(deps *Deps, rep *readDoctorReport) {
	rep.Watch.StatePath = filepath.Join(deps.Dirs.State, alerts.StateFileName)
	data, err := os.ReadFile(rep.Watch.StatePath) //nolint:gosec // alerts.json under the kit state directory
	if errors.Is(err, fs.ErrNotExist) {
		rep.Watch.Note = "watch has not run yet: install it with gh board watch --install"
		return
	}
	if err != nil {
		rep.Watch.Note = "alerts.json unreadable: " + err.Error()
		rep.problem("watch: " + rep.Watch.Note)
		return
	}
	var state domain.AlertState
	if err := json.Unmarshal(data, &state); err != nil {
		rep.Watch.Note = "alerts.json is damaged: " + err.Error()
		rep.problem("watch: " + rep.Watch.Note)
		return
	}
	rep.Watch.Present = true
	rep.Watch.GeneratedAt = readStamp(state.GeneratedAt)
	rep.Watch.Age = deps.Now().Sub(state.GeneratedAt).Truncate(time.Minute).String()
	rep.Watch.Summary = state.Summary
	rep.Watch.Alerts = len(state.Alerts)
}
