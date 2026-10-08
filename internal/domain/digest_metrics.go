package domain

import (
	"strings"
	"time"
)

const (
	digestFirstResponse = "first_response"
	digestAutonomy      = "autonomy"
	digestRunShare      = "run_share"
)

// DigestMetric represents a ratio, or an explicit reason it cannot be measured.
type DigestMetric struct {
	Name        string           `json:"name"`
	Available   bool             `json:"available"`
	Numerator   int              `json:"numerator"`
	Denominator int              `json:"denominator"`
	Value       *float64         `json:"value"`
	Reason      string           `json:"reason,omitempty"`
	Evidence    []MetricEvidence `json:"evidence"`
}

// MetricEvidence retains the source and outcome of every counted item.
type MetricEvidence struct {
	Item    ItemRef   `json:"item"`
	Source  string    `json:"source"`
	ID      string    `json:"id,omitempty"`
	At      time.Time `json:"at"`
	Actor   string    `json:"actor,omitempty"`
	Value   string    `json:"value,omitempty"`
	URL     string    `json:"url,omitempty"`
	Matched bool      `json:"matched"`
	Detail  string    `json:"detail"`
}

func digestMetrics(in DigestInput, out DigestResult) []DigestMetric {
	result := []DigestMetric{}
	if in.Config == nil {
		return result
	}
	for _, name := range in.Config.Digest.Metrics {
		metric := DigestMetric{Name: name, Evidence: []MetricEvidence{}}
		switch name {
		case digestFirstResponse:
			metric = digestResponseMetric(in, out)
		case digestAutonomy:
			metric = digestAutonomyMetric(in, out)
		case digestRunShare:
			metric = digestRunMetric(in, out)
		default:
			metric.Reason = "métrica desconhecida"
		}
		result = append(result, digestFinishMetric(metric))
	}
	return result
}

func digestFinishMetric(metric DigestMetric) DigestMetric {
	if metric.Reason == "" && metric.Denominator == 0 {
		metric.Reason = "nenhum item elegível na semana"
	}
	if metric.Reason == "" {
		value := float64(metric.Numerator) / float64(metric.Denominator)
		metric.Value, metric.Available = &value, true
	}
	return metric
}

func digestTriageMetric(in DigestInput, name string) (DigestMetric, *TriageCapability) {
	m := DigestMetric{Name: name, Evidence: []MetricEvidence{}}
	tri := CapabilitiesOf(in.Config).Triage
	if tri == nil || tri.Label == "" {
		m.Reason = "triagem não mapeada em board.yml"
	}
	return m, tri
}

func digestAutonomyMetric(in DigestInput, out DigestResult) DigestMetric {
	m, tri := digestTriageMetric(in, digestAutonomy)
	if m.Reason != "" {
		return m
	}
	if tri.DecisionField == "" {
		m.Reason = "campo de decisão não mapeado em board.yml"
		return m
	}
	for _, it := range in.Items {
		decision, ok := it.Value(tri.DecisionField)
		if !it.HasLabel(tri.Label) || !ok || strings.TrimSpace(decision.Value) == "" {
			continue
		}
		if decision.UpdatedAt.IsZero() {
			m.Reason = "decisão sem data: " + it.Issue.Ref()
			continue
		}
		if digestInWeek(decision.UpdatedAt, out.Start, out.End) && !decision.UpdatedAt.After(in.Now) {
			matched := digestAutonomous(decision.Value)
			digestCount(&m, MetricEvidence{
				Item: ItemRefOf(it), Source: tri.DecisionField,
				At: decision.UpdatedAt, Actor: decision.Creator, Value: decision.Value, Matched: matched,
				Detail: "decisão registrada na semana",
			})
		}
	}
	return m
}

func digestAutonomous(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ensinar", "encaminhar", "capacidade existente":
		return true
	default:
		return false
	}
}

func digestRunMetric(in DigestInput, out DigestResult) DigestMetric {
	m, tri := digestTriageMetric(in, digestRunShare)
	if m.Reason != "" {
		return m
	}
	for _, it := range in.Items {
		if it.Issue.State == IssueClosed && it.Issue.ClosedAt == nil {
			m.Reason = "item fechado sem data de fechamento: " + it.Issue.Ref()
		}
	}
	for _, it := range digestClosed(in.Items, out.Start, out.End, in.Now) {
		digestCount(&m, MetricEvidence{
			Item: ItemRefOf(it), Source: "closed_at", At: *it.Issue.ClosedAt,
			Value: tri.Label, Matched: it.HasLabel(tri.Label), Detail: "item fechado na semana; rótulo de triagem observado",
		})
	}
	return m
}

func digestCount(metric *DigestMetric, evidence MetricEvidence) {
	metric.Denominator++
	if evidence.Matched {
		metric.Numerator++
	}
	metric.Evidence = append(metric.Evidence, evidence)
}
