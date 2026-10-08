package commands

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/gh-board/internal/audit"
	"github.com/cristianargotti/gh-board/internal/domain"
	"github.com/cristianargotti/gh-board/internal/plan"
)

var planDigestCases = []struct {
	name, status, failure string
	existing              bool
	code                  domain.ExitCode
}{
	{"normal", "on_track", "", false, domain.ExitPlanRequired},
	{"risk", "at_risk", "", false, domain.ExitPlanRequired},
	{"off", "off_track", "", false, domain.ExitPlanRequired},
	{"bad", "green", "", false, domain.ExitUsage},
	{"existing", "on_track", "", true, domain.ExitOK},
	{"history", "on_track", "updates", false, domain.ExitAPI},
	{"items", "on_track", "list", false, domain.ExitAPI},
	{"milestones", "on_track", "milestones", false, domain.ExitAPI},
	{"timeline", "on_track", "timeline", false, domain.ExitAPI},
}

func TestPlanDigest(t *testing.T) {
	for index, tc := range planDigestCases {
		t.Run(tc.name, func(t *testing.T) {
			planTestDigestCase(t, index)
		})
	}
}

func planTestDigestCase(t *testing.T, index int) {
	t.Helper()
	tc := planDigestCases[index]
	deps, reader, writer, _ := planTestDeps(t)
	deps.Config.Config.Digest.Metrics = []string{"first_response"}
	deps.Config.Config.Capabilities.Triage = &domain.TriageCapability{Label: "entrada"}
	reader.items[0].Issue.Labels = []domain.Label{{Name: "entrada"}}
	if tc.failure != "" {
		reader.fail[tc.failure] = errors.New("offline")
	}
	if tc.existing {
		reader.updates = []plan.StatusUpdate{{Body: plan.WeekMarker(planTestNow)}}
	}
	err := planTestExecute(context.Background(), []string{"digest", "post", "--status", tc.status}, deps)
	if domain.CodeOf(err) != tc.code {
		t.Fatalf("error = %v", err)
	}
	if len(writer.calls) != 0 {
		t.Fatal("digest wrote during planning")
	}
	if tc.code == domain.ExitPlanRequired {
		planTestDigestPayload(t, deps, tc.status)
	}
}

func planTestDigestPayload(t *testing.T, deps *Deps, status string) {
	t.Helper()
	p := planTestSaved(t, deps)
	var input domain.StatusUpdateInput
	if err := json.Unmarshal([]byte(p.Steps[0].After), &input); err != nil {
		t.Fatal(err)
	}
	if input.Status != strings.ToUpper(status) || input.StartDate == nil || !strings.Contains(input.Body, plan.WeekMarker(planTestNow)) {
		t.Fatalf("input = %+v", input)
	}
	if !strings.Contains(input.Body, "Resumo da semana") {
		t.Fatal("digest is not in PT-BR")
	}
}

func TestPlanDigestMissingHistory(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "missing", false: "present"}[missing], func(t *testing.T) {
			deps, reader, _, _ := planTestDeps(t)
			if missing {
				deps.Reader = struct{ domain.ProjectReader }{reader}
			}
			err := planTestExecute(context.Background(), []string{"digest", "post"}, deps)
			want := domain.ExitPlanRequired
			if missing {
				want = domain.ExitAPI
			}
			if domain.CodeOf(err) != want {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanDigestExecutionIdempotency(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(map[bool]string{true: "exists", false: "new"}[exists], func(t *testing.T) {
			deps, reader, writer, _ := planTestDeps(t)
			if exists {
				reader.updates = []plan.StatusUpdate{{ID: "old", Body: plan.WeekMarker(planTestNow)}}
			}
			wrapped := &planApplyWriter{ProjectWriter: writer, reader: deps.Reader, state: deps.Dirs.State}
			input := domain.StatusUpdateInput{ProjectID: "PVT_TEST", Body: plan.WeekMarker(planTestNow), StartDate: &planTestNow}
			id, err := wrapped.CreateStatusUpdate(context.Background(), input)
			if err != nil || id == "" {
				t.Fatalf("result = %q, %v", id, err)
			}
			if exists && len(writer.calls) != 0 {
				t.Fatal("duplicate update")
			}
		})
	}
}

func TestPlanDigestLockRefusal(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "locked", true: "no state"}[missing], func(t *testing.T) {
			planTestDigestLockCase(t, missing)
		})
	}
}

func planTestDigestLockCase(t *testing.T, missing bool) {
	t.Helper()
	deps, reader, writer, _ := planTestDeps(t)
	marker := plan.WeekMarker(planTestNow)
	input := domain.StatusUpdateInput{ProjectID: "PVT_TEST", Body: marker, StartDate: &planTestNow}
	wrapped := &planApplyWriter{ProjectWriter: writer, reader: reader, state: deps.Dirs.State}
	if missing {
		wrapped.state = ""
	} else {
		name := fmt.Sprintf("digest-%x.lock", sha256.Sum256([]byte(input.ProjectID+"|"+marker)))
		release, err := audit.Lock(filepath.Join(deps.Dirs.State, plan.JournalDir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := release(); err != nil {
				t.Error(err)
			}
		}()
	}
	_, err := wrapped.CreateStatusUpdate(context.Background(), input)
	if err == nil || len(writer.calls) != 0 {
		t.Fatalf("err %v, writes %v", err, writer.calls)
	}
}
