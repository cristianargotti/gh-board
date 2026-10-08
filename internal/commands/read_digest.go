package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/render"
)

// readMetricFirstResponse is the only metric that needs issue timelines.
const readMetricFirstResponse = "first_response"

func readDigestPrintCommand(deps *Deps) *cobra.Command {
	var week string
	cmd := &cobra.Command{
		Use:     "print",
		Short:   "Print the weekly digest in PT-BR without posting it",
		GroupID: GroupRead,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return readDigestPrint(cmd.Context(), deps, week) },
	}
	cmd.Flags().StringVar(&week, "week", "", "ISO week to print, such as 2026-W41 (default: the week of today)")
	return cmd
}

// readDigestPrint computes the week through the domain digest, the same
// computation digest post publishes, and prints it as board content.
func readDigestPrint(ctx context.Context, deps *Deps, week string) error {
	s, err := readStart(ctx, deps)
	if err != nil {
		return err
	}
	items, err := s.items(ctx)
	if err != nil {
		return err
	}
	input, err := readDigestInput(ctx, s, items, week)
	if err != nil {
		return err
	}
	digest, err := domain.BuildDigest(input)
	if err != nil {
		return err
	}
	digest = readDelimitDigest(digest)
	doc := readDigestDocument(digest, domain.LocationOf(s.cfg))
	doc.Data = readDigestPayloadOf(digest)
	return readRender(deps, doc)
}

// readDigestPayload is the --json form of digest print: the domain result
// with its instants in the UTC form every payload shares.
type readDigestPayload struct {
	GeneratedAt string `json:"generated_at"`
	Start       string `json:"start"`
	End         string `json:"end"`
	RisksAsOf   string `json:"risks_as_of"`
	domain.DigestResult
}

func readDigestPayloadOf(d domain.DigestResult) readDigestPayload {
	return readDigestPayload{
		DigestResult: d, Start: readStamp(d.Start), End: readStamp(d.End),
		GeneratedAt: readStamp(d.GeneratedAt), RisksAsOf: readStamp(d.RisksAsOf),
	}
}

// readDigestInput gathers what the domain needs beyond the items: the
// repository milestones and, for first_response, the issue timelines
// through the narrow timeline port when the adapter offers it. A missing
// timeline leaves the metric unavailable with its reason (F10).
func readDigestInput(ctx context.Context, s *readSession, items []domain.Item, week string) (domain.DigestInput, error) {
	input := domain.DigestInput{
		Config: s.cfg, Project: s.project, Items: items, Now: s.now, Week: week,
		Timelines: map[string]domain.IssueTimeline{},
	}
	if s.cfg.Repository != "" {
		owner, name, err := domain.SplitRepository(s.cfg.Repository)
		if err != nil {
			return input, err
		}
		milestones, err := s.deps.Reader.RepositoryMilestones(ctx, owner, name)
		if err != nil {
			return input, readAPI(err)
		}
		for _, m := range milestones {
			input.Milestones = append(input.Milestones, domain.DigestMilestone{Repository: s.cfg.Repository, Milestone: m})
		}
	}
	reader, ok := s.deps.Reader.(domain.IssueTimelineReader)
	tri := domain.CapabilitiesOf(s.cfg).Triage
	if !ok || tri == nil || !domain.Contains(s.cfg.Digest.Metrics, readMetricFirstResponse) {
		return input, nil
	}
	for _, it := range items {
		if !it.HasLabel(tri.Label) {
			continue
		}
		timeline, err := reader.IssueTimeline(ctx, it.Issue.NodeID)
		if err != nil {
			return input, readAPI(err)
		}
		input.Timelines[it.Issue.NodeID] = timeline
	}
	return input, nil
}

// readDelimitDigest wraps the external titles the structured payload
// carries; the PT-BR text is already delimited by the domain.
func readDelimitDigest(d domain.DigestResult) domain.DigestResult {
	d.Deliveries = readDelimitSummaries(d.Deliveries)
	d.Risks = readDelimitAlerts(d.Risks)
	d.Milestones = append(make([]domain.DigestMilestone, 0, len(d.Milestones)), d.Milestones...)
	for i := range d.Milestones {
		d.Milestones[i].Milestone.Title = render.Title(d.Milestones[i].Milestone.Title)
	}
	for i := range d.Metrics {
		d.Metrics[i].Evidence = readDelimitEvidence(d.Metrics[i].Evidence)
	}
	return d
}

func readDelimitEvidence(in []domain.MetricEvidence) []domain.MetricEvidence {
	out := make([]domain.MetricEvidence, len(in))
	for i, e := range in {
		e.Item.Title = render.Title(e.Item.Title)
		out[i] = e
	}
	return out
}

// readDigestDocument lays the digest out as board content, in PT-BR.
func readDigestDocument(d domain.DigestResult, loc *time.Location) *render.Document {
	doc := render.NewDocument("Resumo da semana " + d.Week)
	head := doc.AddSection("")
	head.AddKeyValue("Período", domain.FormatDay(d.Start, loc)+" a "+domain.FormatDay(d.End.AddDate(0, 0, -1), loc))
	head.AddKeyValue("Projeto", d.Project.String())
	deliveries := doc.AddSection("Entregas")
	if len(d.Deliveries) == 0 {
		deliveries.AddNote("Nenhuma entrega registrada na semana.")
	}
	t := deliveries.SetTable(readColRef, readColStatus, "RESPONSÁVEIS", "TÍTULO")
	for _, it := range d.Deliveries {
		t.AddRow(it.Ref, it.Status, strings.Join(it.Assignees, ", "), it.Title)
	}
	readDigestMilestones(doc.AddSection("Marcos com prazo na semana"), d.Milestones)
	readDigestRisks(doc.AddSection("Riscos observados em "+readStamp(d.RisksAsOf)), d.Risks)
	readDigestMetrics(doc.AddSection("Métricas"), doc.AddSection("Evidências"), d.Metrics)
	doc.AddSection("").AddKeyValue("Gerado em", readStamp(d.GeneratedAt))
	return doc
}

// readDigestRisks writes the alerts the way the team reads them.
func readDigestRisks(sec *render.Section, risks []domain.Alert) {
	if len(risks) == 0 {
		sec.AddNote("Nenhum risco identificado pelas regras declaradas.")
		return
	}
	t := sec.SetTable("REGRA", "SEVERIDADE", "ITEM", "TÍTULO", "MENSAGEM")
	for _, a := range risks {
		item := "quadro"
		if a.Item.NodeID != "" {
			item = fmt.Sprintf("%s#%d", a.Item.Repository, a.Item.Number)
		}
		t.AddRow(a.RuleID, string(a.Severity), item, a.Item.Title, a.Message)
	}
}

func readDigestMilestones(sec *render.Section, entries []domain.DigestMilestone) {
	if len(entries) == 0 {
		sec.AddNote("Nenhum marco com prazo registrado na semana.")
		return
	}
	t := sec.SetTable("REPOSITÓRIO", "MARCO", "PRAZO", "ESTADO")
	for _, e := range entries {
		due := ""
		if e.Milestone.DueOn != nil {
			due = domain.DayText(*e.Milestone.DueOn)
		}
		t.AddRow(e.Repository, fmt.Sprintf("#%d %s", e.Milestone.Number, e.Milestone.Title), due, e.Milestone.State)
	}
}

// readDigestMetrics lists every declared metric with its value or the
// reason it is unavailable, then the evidence behind the counted items.
func readDigestMetrics(sec, evidence *render.Section, metrics []domain.DigestMetric) {
	if len(metrics) == 0 {
		sec.AddNote("Nenhuma métrica declarada em board.yml.")
		return
	}
	for _, m := range metrics {
		sec.AddItem(readMetricLine(m))
	}
	t := evidence.SetTable("MÉTRICA", "ITEM", "FONTE", "QUANDO", "ATOR", "CONTA", "DETALHE")
	for _, m := range metrics {
		for _, e := range m.Evidence {
			counted := "não"
			if e.Matched {
				counted = "sim"
			}
			item := fmt.Sprintf("%s#%d", e.Item.Repository, e.Item.Number)
			t.AddRow(m.Name, item, e.Source, readStamp(e.At), e.Actor, counted, strings.TrimSpace(e.Value+" "+e.Detail))
		}
	}
}

func readMetricLine(m domain.DigestMetric) string {
	if !m.Available || m.Value == nil {
		return fmt.Sprintf("%s: indisponível (%s)", m.Name, m.Reason)
	}
	percent := strings.ReplaceAll(fmt.Sprintf("%.1f", *m.Value*100), ".", ",")
	return fmt.Sprintf("%s: %s%% (%d/%d)", m.Name, percent, m.Numerator, m.Denominator)
}
