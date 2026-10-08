package domain

import (
	"strings"
	"testing"
	"time"
)

var digestAutonomyCases = []struct {
	decision string
	matched  bool
}{
	{"Ensinar", true},
	{" encaminhar ", true},
	{"CAPACIDADE EXISTENTE", true},
	{"Construir", false},
	{"Não ensinar", false},
}

func TestDigestAutonomy(t *testing.T) {
	for _, tc := range digestAutonomyCases {
		t.Run(tc.decision, func(t *testing.T) {
			in, item := digestTestInput(), digestTestItem(1)
			in.Config.Digest.Metrics = []string{"autonomy"}
			item.Values["Decision"] = FieldValue{Value: tc.decision, Creator: "member", UpdatedAt: item.Issue.CreatedAt}
			in.Items = []Item{item}
			out, err := BuildDigest(in)
			m := out.Metrics[0]
			if err != nil || !m.Available || m.Denominator != 1 || m.Evidence[0].Matched != tc.matched {
				t.Fatalf("metric = %+v, error = %v", m, err)
			}
			if !strings.Contains(out.Text, "Evidência") || !strings.Contains(out.Text, strings.TrimSpace(tc.decision)) {
				t.Fatalf("missing evidence: %s", out.Text)
			}
		})
	}
}

var digestUnavailableCases = []struct {
	name, metric, reason string
	edit                 func(*DigestInput, *Item)
}{
	{"no triage", "autonomy", "triagem", func(in *DigestInput, _ *Item) { in.Config.Capabilities.Triage = nil }},
	{"no label", "run_share", "triagem", func(in *DigestInput, _ *Item) { in.Config.Capabilities.Triage.Label = "" }},
	{"no decision mapping", "autonomy", "não mapeado", func(in *DigestInput, _ *Item) { in.Config.Capabilities.Triage.DecisionField = "" }},
	{"undated decision", "autonomy", "sem data", func(_ *DigestInput, it *Item) { it.Values["Decision"] = FieldValue{Value: "Ensinar"} }},
	{"undated closure", "run_share", "sem data", func(_ *DigestInput, it *Item) { it.Issue.State = IssueClosed }},
	{"undated entry", "first_response", "sem data", func(_ *DigestInput, it *Item) { it.Issue.CreatedAt = time.Time{} }},
	{"no members", "first_response", "membros", func(in *DigestInput, _ *Item) { in.Config.Digest.Members = nil }},
	{"missing timeline", "first_response", "histórico", func(_ *DigestInput, _ *Item) {}},
	{"empty cohort", "autonomy", "nenhum item", func(_ *DigestInput, _ *Item) {}},
	{"unknown metric", "unknown", "desconhecida", func(_ *DigestInput, _ *Item) {}},
}

func TestDigestUnavailableMetrics(t *testing.T) {
	for _, tc := range digestUnavailableCases {
		t.Run(tc.name, func(t *testing.T) {
			in, item := digestTestInput(), digestTestItem(1)
			in.Config.Digest.Metrics = []string{tc.metric}
			tc.edit(&in, &item)
			in.Items = []Item{item}
			out, err := BuildDigest(in)
			m := out.Metrics[0]
			if err != nil || m.Available || m.Value != nil || !strings.Contains(m.Reason, tc.reason) {
				t.Fatalf("metric = %+v, error = %v", m, err)
			}
			if !strings.Contains(out.Text, "indisponivel (") {
				t.Fatalf("text = %s", out.Text)
			}
		})
	}
}

var digestRunCases = []struct {
	name       string
	labels     []Label
	num, denom int
}{
	{"triage", []Label{{Name: "intake"}}, 1, 1},
	{"delivery", nil, 0, 1},
	{"case insensitive label", []Label{{Name: "INTAKE"}}, 1, 1},
}

func TestDigestRunShare(t *testing.T) {
	for _, tc := range digestRunCases {
		t.Run(tc.name, func(t *testing.T) {
			in, item := digestTestInput(), digestTestItem(1)
			in.Config.Digest.Metrics = []string{"run_share"}
			at := digestTestTime("2026-10-08T15:00:00Z")
			item.Issue.State, item.Issue.ClosedAt, item.Issue.Labels = IssueClosed, &at, tc.labels
			in.Items = []Item{item}
			out, err := BuildDigest(in)
			m := out.Metrics[0]
			if err != nil || !m.Available || m.Numerator != tc.num || m.Denominator != tc.denom {
				t.Fatalf("metric = %+v, error = %v", m, err)
			}
		})
	}
}

var digestDecisionWindowCases = []struct {
	name, date string
	eligible   bool
}{
	{"in week", "2026-10-05T03:00:00Z", true},
	{"previous week", "2026-10-05T02:59:59Z", false},
	{"next week", "2026-10-12T03:00:00Z", false},
	{"future", "2026-10-11T12:00:00Z", false},
}

func TestDigestDecisionWindow(t *testing.T) {
	for _, tc := range digestDecisionWindowCases {
		t.Run(tc.name, func(t *testing.T) {
			in, it := digestTestInput(), digestTestItem(1)
			in.Now = digestTestTime("2026-10-09T15:00:00Z")
			in.Config.Digest.Metrics = []string{"autonomy"}
			it.Values["Decision"] = FieldValue{Value: "Ensinar", UpdatedAt: digestTestTime(tc.date)}
			in.Items = []Item{it}
			out, err := BuildDigest(in)
			if err != nil || out.Metrics[0].Available != tc.eligible {
				t.Fatalf("metric = %+v, error = %v", out.Metrics, err)
			}
		})
	}
}
