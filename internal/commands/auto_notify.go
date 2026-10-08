package commands

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// autoNotifyLedgerName records when watch notified, under the state
// directory, so that the hourly budget spans runs.
const autoNotifyLedgerName = "notified.json"

// autoNotifyBudget sends the pending alerts within the hourly budget of
// alerts.MaxToastsPerHour, most severe first, and records each send. The
// rest stays in alerts.json, where the mod shows it.
func autoNotifyBudget(ctx context.Context, stateDir string, pending []domain.Alert, now time.Time, out io.Writer) (int, error) {
	if len(pending) == 0 {
		return 0, nil
	}
	path := filepath.Join(stateDir, autoNotifyLedgerName)
	sent := autoReadLedger(path, now.Add(-time.Hour))
	budget := alerts.MaxToastsPerHour - len(sent)
	if budget <= 0 {
		return 0, nil
	}
	chosen := autoMostSevere(pending, budget)
	if err := autoNotify(ctx, chosen, alerts.NotifyOptions{Out: out, Dir: stateDir}); err != nil {
		return 0, err
	}
	for range chosen {
		sent = append(sent, now)
	}
	data, err := json.Marshal(sent)
	if err != nil {
		return len(chosen), err
	}
	return len(chosen), audit.WriteAtomic(path, data)
}

// autoReadLedger returns the send instants after since. A missing or
// damaged ledger counts as empty: a broken file must never silence the
// alerts.
func autoReadLedger(path string, since time.Time) []time.Time {
	data, err := os.ReadFile(path) //nolint:gosec // ledger under the state directory
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	var all []time.Time
	if len(data) > 0 && json.Unmarshal(data, &all) != nil {
		return nil
	}
	recent := all[:0]
	for _, t := range all {
		if t.After(since) {
			recent = append(recent, t)
		}
	}
	return recent
}

// autoMostSevere returns up to n alerts, highest severity first, keeping
// the evaluation order inside a severity.
func autoMostSevere(pending []domain.Alert, n int) []domain.Alert {
	sorted := append([]domain.Alert(nil), pending...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Severity.Rank() > sorted[j].Severity.Rank()
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}
