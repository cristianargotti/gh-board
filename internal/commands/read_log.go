package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readLogDefaultSince is the window of log without --since.
const readLogDefaultSince = "7d"

// readLogPayload is the --json form of log.
type readLogPayload struct {
	GeneratedAt string        `json:"generated_at"`
	Since       string        `json:"since"`
	StateDir    string        `json:"state_dir"`
	Entries     []audit.Entry `json:"entries"`
}

func readLogCommand(deps *Deps) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:     "log",
		Short:   "Local audit log of the writes this user made through the kit",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(_ *cobra.Command, _ []string) error { return readLog(deps, since) },
	}
	cmd.Flags().StringVar(&since, "since", readLogDefaultSince, "window: a duration (24h, 7d, 2w), a date (YYYY-MM-DD) or an RFC 3339 instant")
	return cmd
}

func readLog(deps *Deps, since string) error {
	now := deps.Now()
	from, err := readParseSince(since, now)
	if err != nil {
		return err
	}
	entries, err := audit.Query(deps.Dirs.State, from)
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	payload := readLogPayload{GeneratedAt: readStamp(now), Since: readStamp(from), StateDir: deps.Dirs.State, Entries: entries}
	doc := render.NewDocument("Audit log")
	sec := doc.AddSection("")
	sec.AddKeyValue("Since", payload.Since).AddKeyValue("State dir", payload.StateDir)
	sec.AddKeyValue("Entries", fmt.Sprint(len(entries)))
	t := sec.SetTable("AT", "ACTOR", "COMMAND", "TARGETS", "RESULT", "DRY RUN", readColReason, "PLAN")
	for _, e := range entries {
		t.AddRow(readStamp(e.At), readLogActor(e), e.Command, readLogTargets(e), readLogResult(e), readYesNo(e.DryRun), e.Reason, e.PlanID)
	}
	doc.AddSection("").AddKeyValue(readKeyGenerated, payload.GeneratedAt)
	doc.Data = payload
	return readRender(deps, doc)
}

// readLogTargets prefers the owner/repo#n names over the node ids.
func readLogTargets(e audit.Entry) string {
	if len(e.Items) > 0 {
		return strings.Join(e.Items, ", ")
	}
	return strings.Join(e.Targets, ", ")
}

// readLogActor names the actor, or says the command failed before the
// identity was resolved.
func readLogActor(e audit.Entry) string {
	if e.Actor == "" {
		return "(unresolved)"
	}
	return e.Actor
}

func readLogResult(e audit.Entry) string {
	if e.Error != "" {
		return e.Result + ": " + e.Error
	}
	return e.Result
}

// readParseSince reads the --since value: a duration with the extra day
// and week units, a date, or an RFC 3339 instant. Dates are read in the
// process local time, the way the person typed them.
func readParseSince(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		s = readLogDefaultSince
	}
	if days, ok := readDaysOf(s); ok {
		return now.AddDate(0, 0, -days), nil
	}
	if d, err := time.ParseDuration(s); err == nil && d >= 0 {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation(domain.DateLayout, s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, domain.Errorf(domain.ExitUsage, "--since %q: expected a duration (24h, 7d, 2w), a date (YYYY-MM-DD) or an RFC 3339 instant", s)
}

// readDaysOf reads the day and week units a duration lacks.
func readDaysOf(s string) (int, bool) {
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 {
		return 0, false
	}
	switch unit {
	case 'd':
		return n, true
	case 'w':
		return n * 7, true
	}
	return 0, false
}
