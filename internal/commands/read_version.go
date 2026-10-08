package commands

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/render"
)

// readVersionPayload is the --json form of version.
type readVersionPayload struct {
	Version string `json:"version"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func readVersionCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Kit version and build platform",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(_ *cobra.Command, _ []string) error { return readVersion(deps) },
	}
}

func readVersion(deps *Deps) error {
	payload := readVersionPayload{Version: deps.Version, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	if payload.Version == "" {
		payload.Version = "dev"
	}
	doc := render.NewDocument("gh board " + payload.Version)
	doc.AddSection("").AddKeyValue("Version", payload.Version).AddKeyValue("Go", payload.Go).
		AddKeyValue("Platform", payload.OS+"/"+payload.Arch)
	doc.Data = payload
	return readRender(deps, doc)
}
