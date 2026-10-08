package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readEpicsPayload is the --json form of epics.
type readEpicsPayload struct {
	GeneratedAt string                `json:"generated_at"`
	Available   bool                  `json:"available"`
	Reason      string                `json:"reason,omitempty"`
	Epics       []domain.EpicProgress `json:"epics"`
}

func readEpicsCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "epics",
		Short:   "Open epics with sub-issue progress, dates and owner",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readEpics(cmd.Context(), deps) },
	}
}

func readEpics(ctx context.Context, deps *Deps) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	payload := readEpicsPayload{GeneratedAt: readStamp(s.now), Epics: []domain.EpicProgress{}}
	doc := render.NewDocument("Epics " + s.project.Ref.String())
	sec := doc.AddSection("")
	if caps := domain.CapabilitiesOf(s.cfg); caps.Epic == nil || caps.Epic.IssueType == "" {
		payload.Reason = "epic " + readUnavailable
		sec.AddNote(payload.Reason)
	} else {
		items, err := s.summaries(ctx)
		if err != nil {
			return err
		}
		payload.Available = true
		payload.Epics = readDelimitEpics(domain.EpicsProgress(s.cfg, items))
		readEpicTable(sec, payload.Epics)
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, payload.GeneratedAt)
	doc.Data = payload
	return readRender(deps, doc)
}
