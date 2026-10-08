package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readSearchPayload is the --json form of search.
type readSearchPayload struct {
	GeneratedAt string          `json:"generated_at"`
	Query       string          `json:"query"`
	Total       int             `json:"total"`
	Items       []readSearchHit `json:"items"`
	// Omitted counts the board items that are not issues.
	Omitted int `json:"omitted"`
}

type readSearchHit struct {
	domain.ItemSummary
	State   string   `json:"state"`
	Matched []string `json:"matched"`
}

func readSearchCommand(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:         "search <query>",
		Annotations: map[string]string{mcpArgsAnnotation: "query: Text searched in titles, bodies and labels of the board items"},
		Short:       "Find items whose title, body, reference or labels contain the query",
		GroupID:     GroupRead,
		Args:        cobra.ExactArgs(1),
		RunE:        func(cmd *cobra.Command, args []string) error { return readSearch(cmd.Context(), deps, args[0]) },
	}
}

// readSearch matches the query as a case-insensitive substring over every
// item of the board, archived ones excluded.
func readSearch(ctx context.Context, deps *Deps, query string) error {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return domain.Errorf(domain.ExitUsage, "search needs a non-empty query")
	}
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	items, err := s.items(ctx)
	if err != nil {
		return err
	}
	payload := readSearchPayload{GeneratedAt: readStamp(s.now), Query: render.Sanitize(query, 0), Items: []readSearchHit{}}
	for _, it := range items {
		if it.Archived {
			continue
		}
		if matched := readSearchMatch(it, needle); len(matched) > 0 {
			sum := domain.SummarizeItem(s.cfg, it)
			sum.Title = render.Title(sum.Title)
			payload.Items = append(payload.Items, readSearchHit{ItemSummary: sum, State: string(it.Issue.State), Matched: matched})
		}
	}
	payload.Total, payload.Omitted = len(payload.Items), s.skipped
	doc := render.NewDocument("Search " + s.project.Ref.String())
	sec := doc.AddSection("")
	sec.AddKeyValue("Query", payload.Query).AddKeyValue("Matches", fmt.Sprint(payload.Total))
	if note := readSkippedNote(payload.Omitted); note != "" {
		sec.AddNote(note)
	}
	t := sec.SetTable(readColRef, readColState, readColStatus, "MATCHED", readColTitle)
	for _, hit := range payload.Items {
		t.AddRow(hit.Ref, hit.State, hit.Status, strings.Join(hit.Matched, ", "), hit.Title)
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, payload.GeneratedAt)
	doc.Data = payload
	return readRender(deps, doc)
}

// readSearchMatch names the parts of the item that contain the needle.
func readSearchMatch(it domain.Item, needle string) []string {
	var matched []string
	if strings.Contains(strings.ToLower(it.Issue.Title), needle) {
		matched = append(matched, "title")
	}
	if strings.Contains(strings.ToLower(it.Issue.Body), needle) {
		matched = append(matched, "body")
	}
	if strings.Contains(strings.ToLower(it.Issue.Ref()), needle) {
		matched = append(matched, "ref")
	}
	for _, l := range it.Issue.Labels {
		if strings.Contains(strings.ToLower(l.Name), needle) {
			matched = append(matched, "label")
			break
		}
	}
	return matched
}
