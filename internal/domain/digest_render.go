package domain

import (
	"fmt"
	"strings"
	"time"
)

func digestText(out DigestResult) string {
	lines := []string{
		"Resumo da semana " + out.Week,
		out.Start.Format(DayLayout) + " a " + out.End.AddDate(0, 0, -1).Format(DayLayout), "", "Entregas",
	}
	lines = append(lines, digestDeliveryLines(out.Deliveries)...)
	lines = append(lines, "", "Marcos com prazo na semana")
	lines = append(lines, digestMilestoneLines(out.Milestones)...)
	lines = append(lines, "", "Riscos observados em "+out.RisksAsOf.Format(time.RFC3339))
	lines = append(lines, digestRiskLines(out.Risks)...)
	lines = append(lines, "", "Métricas")
	if len(out.Metrics) == 0 {
		lines = append(lines, "Nenhuma métrica declarada em board.yml.")
	}
	for _, metric := range out.Metrics {
		lines = append(lines, "- "+digestMetricLine(metric))
		for _, evidence := range metric.Evidence {
			lines = append(lines, digestEvidenceLine(evidence))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func digestDeliveryLines(items []ItemSummary) []string {
	if len(items) == 0 {
		return []string{"Nenhuma entrega registrada na semana."}
	}
	lines := []string{}
	for _, it := range items {
		lines = append(lines, "- "+CleanText(it.Ref, 0)+": "+Delimit("title", CleanTitle(it.Title)))
	}
	return lines
}

func digestMilestoneLines(entries []DigestMilestone) []string {
	if len(entries) == 0 {
		return []string{"Nenhum marco com prazo registrado na semana."}
	}
	lines := []string{}
	for _, entry := range entries {
		m := entry.Milestone
		line := fmt.Sprintf("- %s#%d: %s (%s)", CleanText(entry.Repository, 0), m.Number,
			Delimit("title", CleanTitle(m.Title)), CleanText(m.State, 0))
		lines = append(lines, line)
	}
	return lines
}

func digestRiskLines(alerts []Alert) []string {
	if len(alerts) == 0 {
		return []string{"Nenhum risco identificado pelas regras declaradas."}
	}
	lines := []string{}
	for _, alert := range alerts {
		lines = append(lines, fmt.Sprintf("- %s#%d: %s", CleanText(alert.Item.Repository, 0), alert.Item.Number,
			Delimit("alert", CleanText(alert.Message, ExcerptLimit))))
	}
	return lines
}

func digestEvidenceLine(e MetricEvidence) string {
	outcome := "fora do numerador"
	if e.Matched {
		outcome = "incluído no numerador"
	}
	return fmt.Sprintf("  - Evidência %s#%d: %s; %s; %s; %s; %s", CleanText(e.Item.Repository, 0), e.Item.Number,
		CleanText(e.Source, 0), e.At.Format(time.RFC3339), CleanText(e.Actor, 0), outcome,
		Delimit("evidence", CleanText(strings.TrimSpace(e.Value+" "+e.Detail), ExcerptLimit)))
}
