package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

func readStandupCommand(deps *Deps) *cobra.Command {
	var forLogin string
	cmd := &cobra.Command{
		Use:     "standup",
		Short:   "Per person: moved yesterday, active today, blocked",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readStandup(cmd.Context(), deps, forLogin) },
	}
	cmd.Flags().StringVar(&forLogin, "for", "", "only this login")
	return cmd
}

// readStandup computes the standup through the domain, which takes the
// status value timestamp as the evidence of movement.
func readStandup(ctx context.Context, deps *Deps, forLogin string) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	items, err := s.summaries(ctx)
	if err != nil {
		return err
	}
	result := domain.BuildStandup(domain.StandupInput{Config: s.cfg, Items: items, Now: s.now, For: forLogin})
	for i := range result.People {
		p := &result.People[i]
		p.MovedYesterday = readDelimitSummaries(p.MovedYesterday)
		p.ActiveToday = readDelimitSummaries(p.ActiveToday)
		p.Blocked = readDelimitSummaries(p.Blocked)
	}
	payload := readStandupPayload{StandupResult: result, GeneratedAt: readStamp(result.GeneratedAt)}
	doc := render.NewDocument("Standup " + result.Today)
	head := doc.AddSection("")
	head.AddKeyValue("Yesterday", result.Yesterday).AddKeyValue("Today", result.Today)
	if domain.CapabilitiesOf(s.cfg).Status == nil {
		head.AddNote("active today " + readUnavailable + ": status classes are needed")
	}
	if len(result.People) == 0 {
		head.AddNote("nobody moved, is active or is blocked")
	}
	for _, p := range result.People {
		readStandupPerson(doc.AddSection(p.Login), p)
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, payload.GeneratedAt)
	doc.Data = payload
	return readRender(deps, doc)
}

// readStandupPayload is the --json form of standup: the domain result
// with the generated_at stamp in the UTC form every payload shares.
type readStandupPayload struct {
	GeneratedAt string `json:"generated_at"`
	domain.StandupResult
}

func readStandupPerson(sec *render.Section, p domain.StandupPerson) {
	t := sec.SetTable(readColKind, readColRef, readColStatus, readColTitle)
	groups := []struct {
		kind  string
		items []domain.ItemSummary
	}{{"moved yesterday", p.MovedYesterday}, {"active today", p.ActiveToday}, {"blocked", p.Blocked}}
	for _, g := range groups {
		for _, it := range g.items {
			t.AddRow(g.kind, it.Ref, it.Status, it.Title)
		}
	}
	if len(t.Rows) == 0 {
		sec.AddNote("nothing moved yesterday, nothing active, nothing blocked")
	}
}
