package plan_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

type uncertainWriter struct {
	*fakeWriter
	reader *fakeReader
}

func (w uncertainWriter) AddComment(ctx context.Context, id, body string) (domain.Comment, error) {
	comment, _ := w.fakeWriter.AddComment(ctx, id, body)
	comment.Body = body
	w.reader.comments[id] = append(w.reader.comments[id], comment)
	return domain.Comment{}, domain.ErrAPI
}

func (w uncertainWriter) CreateStatusUpdate(ctx context.Context, in domain.StatusUpdateInput) (string, error) {
	id, _ := w.fakeWriter.CreateStatusUpdate(ctx, in)
	w.reader.updates = append(w.reader.updates, plan.StatusUpdate{ID: id, Body: in.Body})
	return "", domain.ErrAPI
}

func TestApplyRecoversPostedComment(t *testing.T) {
	r, w := newReader(), newWriter()
	p := newPlan(t, step(domain.OpAddComment, "", "Feito."))
	checkUncertainRecovery(t, r, w, p)
	if comments := r.comments["I_1"]; len(comments) != 1 || strings.Count(comments[0].Body, plan.Marker(p.ID, 0)) != 1 {
		t.Fatalf("comments = %+v", comments)
	}
}

func TestApplyRecoversPostedDigest(t *testing.T) {
	r, w := newReader(), newWriter()
	in := domain.StatusUpdateInput{Body: "Resumo", Status: "ON_TRACK"}
	p := newPlan(t, creation(t, domain.OpCreateStatusUpdate, in))
	checkUncertainRecovery(t, r, w, p)
	if len(w.updates) != 1 || strings.Count(w.updates[0].Body, plan.WeekMarker(p.CreatedAt)) != 1 {
		t.Fatalf("digest inputs = %+v", w.updates)
	}
}

func checkUncertainRecovery(t *testing.T, r *fakeReader, w *fakeWriter, p domain.Plan) {
	t.Helper()
	opts, _ := applyOptions(t)
	ports := newPorts(r, uncertainWriter{fakeWriter: w, reader: r})
	if _, err := plan.Apply(context.Background(), ports, p, opts); domain.CodeOf(err) != domain.ExitAPI {
		t.Fatalf("initial apply = %v", err)
	}
	result, err := plan.Apply(context.Background(), ports, p, opts)
	if err != nil || len(result.Skipped) != 1 || len(w.calls) != 1 {
		t.Fatalf("resume = %+v, %v, writes %v", result, err, w.calls)
	}
	entries := journalOf(t, opts.StateDir, p.ID)
	if len(entries) != 2 || entries[0].Status != plan.StatusFailed || entries[1].Status != plan.StatusSkipped || entries[1].Created == "" {
		t.Fatalf("journal = %+v", entries)
	}
	log := auditOf(t, opts.StateDir)
	if len(log) != 2 || log[0].Result != "failed" || log[1].Result != "applied" {
		t.Fatalf("audit = %+v", log)
	}
}

var digestMarkerCases = []struct {
	name   string
	marked bool
}{
	{name: "adds marker"},
	{name: "retains marker", marked: true},
}

func TestApplyDigestUsesReportingWeekAcrossYearBoundary(t *testing.T) {
	for _, tc := range digestMarkerCases {
		t.Run(tc.name, func(t *testing.T) { checkDigestWeek(t, tc.marked) })
	}
}

func checkDigestWeek(t *testing.T, marked bool) {
	t.Helper()
	start := time.Date(2020, 12, 28, 0, 0, 0, 0, time.UTC)
	at := time.Date(2021, 1, 3, 23, 55, 0, 0, time.UTC)
	in := domain.StatusUpdateInput{Body: "Resumo", StartDate: &start}
	marker := "<!-- gh-board digest=2020-W53 -->"
	if marked {
		in.Body += "\n\n" + marker
	}
	p := newPlan(t, creation(t, domain.OpCreateStatusUpdate, in))
	p.CreatedAt, p.ExpiresAt = at, at.Add(domain.PlanExpiry)
	p, err := p.Seal()
	if err != nil {
		t.Fatal(err)
	}
	opts, _ := applyOptions(t)
	w := newWriter()
	ports := newPorts(newReader(), w)
	ports.Clock = fakeClock{at.Add(10 * time.Minute)}
	if _, err := plan.Apply(context.Background(), ports, p, opts); err != nil {
		t.Fatal(err)
	}
	if len(w.updates) != 1 || strings.Count(w.updates[0].Body, marker) != 1 || w.updates[0].ProjectID != project.NodeID {
		t.Fatalf("digest inputs = %+v", w.updates)
	}
}

var digestReferenceCases = []execCase{
	{name: "unresolved project", step: domain.Step{Operation: domain.OpCreateStatusUpdate, After: `{"ProjectID":"step:0"}`}, wantErr: domain.ErrUsage},
	{name: "created project", step: domain.Step{Operation: domain.OpCreateStatusUpdate, After: `{"ProjectID":"step:0","Status":"ON_TRACK"}`}, created: map[int]string{0: "PVT_new"}, wantCalls: []string{"CreateStatusUpdate PVT_new ON_TRACK"}, wantStatus: plan.StatusDone, wantNode: "PVTSU_1"},
}

func TestDigestProjectReferences(t *testing.T) {
	runExecCases(t, digestReferenceCases)
}
