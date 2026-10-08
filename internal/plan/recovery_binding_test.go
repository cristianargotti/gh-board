package plan

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var bindingCases = []struct {
	name   string
	target domain.Target
	done   map[int]JournalEntry
	want   domain.Target
	code   domain.ExitCode
}{
	{name: "literal", target: domain.Target{NodeID: "I_1"}, want: domain.Target{NodeID: "I_1"}},
	{name: "pending creation", target: domain.Target{NodeID: Ref(0), ProjectItemID: Ref(1)}, want: domain.Target{NodeID: Ref(0), ProjectItemID: Ref(1)}},
	{name: "done creation", target: domain.Target{NodeID: Ref(0)}, done: map[int]JournalEntry{0: {Created: "I_new"}}, want: domain.Target{NodeID: "I_new"}},
	{name: "skipped creation", target: domain.Target{ProjectItemID: Ref(1)}, done: map[int]JournalEntry{1: {Status: StatusSkipped, Created: "PVTI_existing"}}, want: domain.Target{ProjectItemID: "PVTI_existing"}},
	{name: "missing node binding", target: domain.Target{NodeID: Ref(0)}, done: map[int]JournalEntry{0: {}}, code: domain.ExitApplyRefused},
	{name: "missing item binding", target: domain.Target{ProjectItemID: Ref(1)}, done: map[int]JournalEntry{1: {}}, code: domain.ExitApplyRefused},
	{name: "nested binding", target: domain.Target{NodeID: Ref(0)}, done: map[int]JournalEntry{0: {Created: Ref(1)}}, code: domain.ExitApplyRefused},
	{name: "malformed", target: domain.Target{NodeID: "step:bad"}, code: domain.ExitApplyRefused},
	{name: "negative", target: domain.Target{NodeID: Ref(-1)}, code: domain.ExitApplyRefused},
	{name: "self reference", target: domain.Target{NodeID: Ref(2)}, code: domain.ExitApplyRefused},
	{name: "forward reference", target: domain.Target{NodeID: Ref(3)}, code: domain.ExitApplyRefused},
}

func TestJournalTargetBindings(t *testing.T) {
	for _, tc := range bindingCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bindTarget(domain.Step{Index: 2, Target: tc.target}, tc.done)
			if domain.CodeOf(err) != tc.code || (err == nil && got != tc.want) {
				t.Fatalf("binding = %+v, %v; want %+v, exit %d", got, err, tc.want, tc.code)
			}
		})
	}
}
