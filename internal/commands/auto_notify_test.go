package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/alerts"
	"github.com/cristianargotti/gh-board/internal/domain"
)

// autoCaptureNotify replaces the notifier and returns the alerts it got.
func autoCaptureNotify(t *testing.T, err error) *[]domain.Alert {
	t.Helper()
	var got []domain.Alert
	autoNotify = func(_ context.Context, alerts []domain.Alert, _ alerts.NotifyOptions) error {
		got = append(got, alerts...)
		return err
	}
	t.Cleanup(func() { autoNotify = alerts.Notify })
	return &got
}

func autoAlert(rule string, sev domain.Severity, n int) domain.Alert {
	return domain.Alert{RuleID: rule, Severity: sev, Item: domain.ItemRef{NodeID: "I_" + rule, Repository: "o/r", Number: n}}
}

func autoWriteLedger(t *testing.T, dir string, instants []time.Time) {
	t.Helper()
	data, err := json.Marshal(instants)
	if err != nil {
		t.Fatal(err)
	}
	autoWriteTestFile(t, filepath.Join(dir, autoNotifyLedgerName), data)
}

func TestAutoNotifyBudget(t *testing.T) {
	dir := t.TempDir()
	got := autoCaptureNotify(t, nil)
	pending := []domain.Alert{
		autoAlert("overdue", domain.SeverityInfo, 1),
		autoAlert("blocked", domain.SeverityCritical, 2),
		autoAlert("stale_active", domain.SeverityWarning, 3),
		autoAlert("triage_sla", domain.SeverityCritical, 4),
	}
	autoWriteLedger(t, dir, []time.Time{autoTestNow.Add(-2 * time.Hour), autoTestNow.Add(-10 * time.Minute), autoTestNow.Add(-20 * time.Minute), autoTestNow.Add(-30 * time.Minute)})
	sent, err := autoNotifyBudget(context.Background(), dir, pending, autoTestNow, &bytes.Buffer{})
	if err != nil || sent != 2 || len(*got) != 2 {
		t.Fatalf("sent = %d, got %d, err %v", sent, len(*got), err)
	}
	if (*got)[0].RuleID != "blocked" || (*got)[1].RuleID != "triage_sla" {
		t.Fatalf("order = %v", *got)
	}
	if ledger := autoReadLedger(filepath.Join(dir, autoNotifyLedgerName), autoTestNow.Add(-time.Hour)); len(ledger) != 5 {
		t.Fatalf("ledger = %v", ledger)
	}
	if sent, err := autoNotifyBudget(context.Background(), dir, pending, autoTestNow, &bytes.Buffer{}); err != nil || sent != 0 {
		t.Fatalf("budget exhausted: sent %d, err %v", sent, err)
	}
	if sent, err := autoNotifyBudget(context.Background(), t.TempDir(), nil, autoTestNow, &bytes.Buffer{}); err != nil || sent != 0 {
		t.Fatalf("nothing pending: sent %d, err %v", sent, err)
	}
}

func TestAutoNotifyBudgetErrorsAndDamage(t *testing.T) {
	dir := t.TempDir()
	autoCaptureNotify(t, errors.New("no notifier"))
	_, err := autoNotifyBudget(context.Background(), dir, []domain.Alert{autoAlert("overdue", domain.SeverityInfo, 1)}, autoTestNow, &bytes.Buffer{})
	if err == nil {
		t.Fatal("notifier error must propagate")
	}
	path := filepath.Join(dir, autoNotifyLedgerName)
	autoWriteTestFile(t, path, []byte("{broken"))
	if ledger := autoReadLedger(path, autoTestNow); ledger != nil {
		t.Fatalf("damaged ledger = %v", ledger)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if ledger := autoReadLedger(path, autoTestNow); ledger != nil {
		t.Fatalf("missing ledger = %v", ledger)
	}
	if ledger := autoReadLedger(dir, autoTestNow); ledger != nil {
		t.Fatalf("unreadable ledger = %v", ledger)
	}
	if got := autoMostSevere(nil, 3); len(got) != 0 {
		t.Fatalf("most severe of nothing = %v", got)
	}
}
