package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var codeCases = []struct {
	name string
	err  error
	want domain.ExitCode
}{
	{"nil", nil, domain.ExitOK},
	{"usage", domain.ErrUsage, domain.ExitUsage},
	{"not found wrapped", fmt.Errorf("item: %w", domain.ErrNotFound), domain.ExitNotFound},
	{"ambiguous", domain.ErrAmbiguous, domain.ExitNotFound},
	{"policy", domain.ErrPolicy, domain.ExitPolicy},
	{"drift", domain.ErrDrift, domain.ExitDrift},
	{"plan required", domain.ErrPlanRequired, domain.ExitPlanRequired},
	{"apply refused", domain.ErrApplyRefused, domain.ExitApplyRefused},
	{"api", domain.ErrAPI, domain.ExitAPI},
	{"explicit code wins", domain.NewError(domain.ExitAPI, domain.ErrPolicy), domain.ExitAPI},
	{"errorf keeps code", domain.Errorf(domain.ExitDrift, "field %s: %w", "Status", domain.ErrDrift), domain.ExitDrift},
	{"explicit wrapped", fmt.Errorf("outer: %w", domain.NewError(domain.ExitPolicy, nil)), domain.ExitPolicy},
	{"unknown", errors.New("boom"), domain.ExitUsage},
	{"not implemented", domain.ErrNotImplemented, domain.ExitUsage},
}

func TestCodeOf(t *testing.T) {
	for _, tc := range codeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.CodeOf(tc.err); got != tc.want {
				t.Fatalf("CodeOf(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestErrorMethods(t *testing.T) {
	e := domain.NewError(domain.ExitPolicy, domain.ErrPolicy)
	if e.Error() != domain.ErrPolicy.Error() {
		t.Fatalf("Error() = %q", e.Error())
	}
	if !errors.Is(e, domain.ErrPolicy) {
		t.Fatal("Unwrap must expose the cause")
	}
	if e.ExitCode() != domain.ExitPolicy {
		t.Fatalf("ExitCode() = %v", e.ExitCode())
	}
	if bare := domain.NewError(domain.ExitAPI, nil); bare.Error() != "api" {
		t.Fatalf("Error() without cause = %q", bare.Error())
	}
}

var codeNames = map[domain.ExitCode]string{
	domain.ExitOK: "ok", domain.ExitUsage: "usage", domain.ExitNotFound: "not-found",
	domain.ExitPolicy: "policy", domain.ExitDrift: "drift", domain.ExitPlanRequired: "plan-required",
	domain.ExitApplyRefused: "apply-refused", domain.ExitAPI: "api", domain.ExitCode(42): "exit-42",
}

func TestExitCodeString(t *testing.T) {
	for code, want := range codeNames {
		if got := code.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(code), got, want)
		}
	}
}
