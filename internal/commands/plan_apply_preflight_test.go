package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var planPreflightCases = []struct {
	name      string
	configure func(*planApplyWriter, *planTestReader)
	code      domain.ExitCode
}{
	{"writer", func(w *planApplyWriter, _ *planTestReader) { w.ProjectWriter = nil }, domain.ExitUsage},
	{"lister", func(w *planApplyWriter, r *planTestReader) {
		w.reader = struct{ domain.ProjectReader }{r}
		w.p.Steps = []domain.Step{{Operation: domain.OpCreateStatusUpdate}}
	}, domain.ExitAPI},
	{"repository", func(w *planApplyWriter, _ *planTestReader) {
		w.p.Steps = []domain.Step{{Target: domain.Target{Repository: "bad"}}}
	}, domain.ExitUsage},
	{"permission API", func(_ *planApplyWriter, r *planTestReader) { r.fail["permission"] = errors.New("offline") }, domain.ExitAPI},
	{"permission", func(_ *planApplyWriter, r *planTestReader) { r.permission = domain.PermissionRead }, domain.ExitPolicy},
	{"project API", func(_ *planApplyWriter, r *planTestReader) { r.fail["discover"] = errors.New("offline") }, domain.ExitAPI},
	{"project", func(_ *planApplyWriter, r *planTestReader) { r.project.ViewerRole = domain.RoleReader }, domain.ExitPolicy},
	{"allowed", func(_ *planApplyWriter, _ *planTestReader) {}, domain.ExitOK},
}

func TestPlanApplyPreflight(t *testing.T) {
	for _, tc := range planPreflightCases {
		t.Run(tc.name, func(t *testing.T) {
			_, reader, writer, _ := planTestDeps(t)
			wrapped := &planApplyWriter{ProjectWriter: writer, reader: reader, p: domain.Plan{Project: reader.project.Ref, Steps: []domain.Step{{Target: domain.Target{Repository: "team/work"}}}}}
			tc.configure(wrapped, reader)
			err := wrapped.planApplyPreflight(context.Background())
			if domain.CodeOf(err) != tc.code {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanApplyHostAndFormat(t *testing.T) {
	for _, kind := range []string{"host", "format"} {
		t.Run(kind, func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			p := domain.Plan{Host: reader.viewer.Host}
			want := domain.ExitUsage
			if kind == "host" {
				p.Host = "other.test"
				want = domain.ExitApplyRefused
			} else {
				deps.Flags.Format = "invalid"
			}
			err := planApplyRun(context.Background(), deps, p, reader.viewer, true)
			if domain.CodeOf(err) != want {
				t.Fatal(err)
			}
		})
	}
}
