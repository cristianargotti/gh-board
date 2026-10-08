package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readMePayload is the --json form of me.
type readMePayload struct {
	GeneratedAt string               `json:"generated_at"`
	Viewer      string               `json:"viewer"`
	Items       []domain.ItemSummary `json:"items"`
}

func readMeCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "me",
		Short:   "My open items on the board",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readMe(cmd.Context(), deps) },
	}
}

func readMe(ctx context.Context, deps *Deps) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	viewer, err := deps.Reader.Viewer(ctx)
	if err != nil {
		return readAPI(err)
	}
	items, err := s.summaries(ctx)
	if err != nil {
		return err
	}
	mine := make([]domain.Item, 0)
	for _, it := range domain.Workable(items) {
		if it.AssignedTo(viewer.Login) {
			mine = append(mine, it)
		}
	}
	payload := readMePayload{
		GeneratedAt: readStamp(s.now), Viewer: viewer.Login,
		Items: readDelimitSummaries(domain.SummarizeItems(s.cfg, mine)),
	}
	doc := render.NewDocument("Open items of " + viewer.Login)
	readSummaryTable(doc.AddSection(""), payload.Items, readColumnsOf(s.cfg))
	doc.AddSection("").AddKeyValue(readKeyGenerated, payload.GeneratedAt)
	doc.Data = payload
	return readRender(deps, doc)
}
