package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/guard"
)

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
