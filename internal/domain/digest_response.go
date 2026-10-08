package domain

import (
	"fmt"
	"strings"
	"time"
)

func digestResponseMetric(in DigestInput, out DigestResult) DigestMetric {
	m, tri := digestTriageMetric(in, digestFirstResponse)
	if m.Reason != "" {
		return m
	}
	if len(in.Config.Digest.Members) == 0 {
		m.Reason = "membros não declarados em board.yml"
		return m
	}
	for _, it := range in.Items {
		if !it.HasLabel(tri.Label) {
			continue
		}
		if it.Issue.CreatedAt.IsZero() {
			m.Reason = "entrada sem data de criação: " + it.Issue.Ref()
			continue
		}
		if !digestInWeek(it.Issue.CreatedAt, out.Start, out.End) || it.Issue.CreatedAt.After(in.Now) {
			continue
		}
		evidence, reason := digestResponseEvidence(in, it, tri)
		if reason != "" {
			m.Reason = reason + ": " + it.Issue.Ref()
			continue
		}
		digestCount(&m, evidence)
	}
	return m
}

func digestResponseEvidence(in DigestInput, it Item, tri *TriageCapability) (MetricEvidence, string) {
	timeline, ok := in.Timelines[it.Issue.NodeID]
	if !ok || !timeline.Complete {
		reason := "histórico completo de comentários e decisões não disponível"
		if timeline.Reason != "" {
			reason += " (" + CleanText(timeline.Reason, ExcerptLimit) + ")"
		}
		return MetricEvidence{}, reason
	}
	events, valid := digestResponseEvents(timeline, tri.DecisionField)
	if !valid {
		return MetricEvidence{}, "histórico com autor ou data ausente"
	}
	first, found := digestFirstMemberEvent(events, in.Config.Digest.Members, it.Issue.CreatedAt, in.Now)
	deadline := digestSLAEnd(it, tri, LocationOf(in.Config))
	if !found && in.Now.Before(deadline) {
		return MetricEvidence{}, "SLA ainda em andamento, sem primeira resposta"
	}
	first.Item = ItemRefOf(it)
	first.Matched = found && first.At.Before(deadline)
	first.Detail = "SLA até " + deadline.Add(-time.Nanosecond).Format(time.RFC3339)
	if !found {
		first.Source, first.At = "timeline", in.Now
		first.Detail += "; nenhuma resposta de membro"
	}
	return first, ""
}

func digestResponseEvents(timeline IssueTimeline, field string) ([]MetricEvidence, bool) {
	events := []MetricEvidence{}
	for _, c := range timeline.Comments {
		if c.CreatedAt.IsZero() || c.Author == "" {
			return nil, false
		}
		events = append(events, MetricEvidence{Source: "comment", ID: c.ID, At: c.CreatedAt, Actor: c.Author, URL: c.URL})
	}
	for _, d := range timeline.Decisions {
		if field == "" || d.Field != field || strings.TrimSpace(d.Value) == "" {
			continue
		}
		if d.CreatedAt.IsZero() || d.Actor == "" {
			return nil, false
		}
		events = append(events, MetricEvidence{Source: "decision", ID: d.ID, At: d.CreatedAt, Actor: d.Actor, Value: d.Value, URL: d.URL})
	}
	return events, true
}

func digestFirstMemberEvent(events []MetricEvidence, members []string, created, now time.Time) (MetricEvidence, bool) {
	var first MetricEvidence
	found := false
	for _, event := range events {
		if event.At.Before(created) || event.At.After(now) || !digestMember(members, event.Actor) {
			continue
		}
		if !found || event.At.Before(first.At) {
			first, found = event, true
		}
	}
	return first, found
}

func digestMember(members []string, login string) bool {
	for _, member := range members {
		if strings.EqualFold(member, login) {
			return true
		}
	}
	return false
}

func digestSLAEnd(it Item, tri *TriageCapability, loc *time.Location) time.Time {
	date := DateOf(it.Issue.CreatedAt, loc)
	if tri.UrgentLabel != "" && it.HasLabel(tri.UrgentLabel) {
		return date.AddDate(0, 0, 1)
	}
	days := tri.SLABusinessDays
	if days <= 0 {
		days = DefaultTriageSLADays
	}
	return AddBusinessDays(date, days, loc).AddDate(0, 0, 1)
}

func digestMetricLine(m DigestMetric) string {
	if !m.Available || m.Value == nil {
		return fmt.Sprintf("%s: indisponivel (%s)", m.Name, CleanText(m.Reason, ExcerptLimit))
	}
	percent := strings.ReplaceAll(fmt.Sprintf("%.1f", *m.Value*100), ".", ",")
	return fmt.Sprintf("%s: %s%% (%d/%d)", m.Name, percent, m.Numerator, m.Denominator)
}
