package domain

import (
	"math"
	"strings"
	"testing"
)

var digestRatioCases = []struct {
	metric     string
	values     []string
	num, denom int
	value      float64
}{
	{"autonomy", []string{"Ensinar", "Encaminhar", "Construir"}, 2, 3, 2.0 / 3.0},
	{"run_share", []string{"intake", "delivery"}, 1, 2, 0.5},
	{"first_response", []string{"on time", "late", "on time"}, 2, 3, 2.0 / 3.0},
}

func digestRatioInput(metric string, values []string) DigestInput {
	in := digestTestInput()
	in.Config.Digest.Metrics = []string{metric}
	for i, value := range values {
		item := digestTestItem(i + 1)
		at := digestTestTime("2026-10-06T12:00:00Z")
		switch metric {
		case "autonomy":
			item.Values["Decision"] = FieldValue{Value: value, UpdatedAt: at}
		case "run_share":
			item.Issue.State, item.Issue.ClosedAt = IssueClosed, &at
			item.Issue.Labels = []Label{{Name: value}}
		case "first_response":
			if value == "late" {
				at = digestTestTime("2026-10-09T12:00:00Z")
			}
			in.Timelines[item.Issue.NodeID] = IssueTimeline{Complete: true, Comments: []Comment{{Author: "member", CreatedAt: at}}}
		}
		in.Items = append(in.Items, item)
	}
	return in
}

func TestDigestMetricRatios(t *testing.T) {
	for _, tc := range digestRatioCases {
		t.Run(tc.metric, func(t *testing.T) {
			out, err := BuildDigest(digestRatioInput(tc.metric, tc.values))
			if err != nil {
				t.Fatal(err)
			}
			m := out.Metrics[0]
			if !m.Available || m.Value == nil || m.Numerator != tc.num || m.Denominator != tc.denom || len(m.Evidence) != tc.denom {
				t.Fatalf("metric = %+v", m)
			}
			if math.Abs(*m.Value-tc.value) > 1e-10 {
				t.Fatalf("ratio = %f, want %f", *m.Value, tc.value)
			}
		})
	}
}

func TestDigestIncompleteCohort(t *testing.T) {
	for _, missing := range []int{1, 2} {
		in := digestRatioInput("first_response", []string{"on time", "on time"})
		delete(in.Timelines, in.Items[missing-1].Issue.NodeID)
		out, err := BuildDigest(in)
		if err != nil || out.Metrics[0].Available || out.Metrics[0].Value != nil {
			t.Fatalf("partial cohort should be unavailable: %+v, %v", out.Metrics, err)
		}
		if !strings.Contains(out.Text, "indisponivel") || !strings.Contains(out.Text, "Evidência") {
			t.Fatalf("missing retained evidence or reason: %s", out.Text)
		}
	}
}
