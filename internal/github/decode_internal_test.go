package github

import (
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var permissionCases = []struct {
	raw  string
	want domain.Permission
}{
	{"ADMIN", domain.PermissionAdmin},
	{"MAINTAIN", domain.PermissionMaintain},
	{"WRITE", domain.PermissionWrite},
	{"TRIAGE", domain.PermissionTriage},
	{"TRIAGE_PLUS", domain.PermissionTriage},
	{"READ", domain.PermissionRead},
	{"", domain.PermissionNone},
	{"SOMETHING_NEW", domain.PermissionNone},
}

func TestToPermission(t *testing.T) {
	for _, tc := range permissionCases {
		if got := toPermission(tc.raw); got != tc.want {
			t.Errorf("toPermission(%q) = %s, want %s", tc.raw, got, tc.want)
		}
	}
}

var roleCases = []struct {
	name      string
	ownerType string
	owner     string
	admin     bool
	canUpdate bool
	viewer    string
	want      domain.Role
}{
	{"organization administrator", ownerOrganization, "org", true, true, "me", domain.RoleAdmin},
	{"organization writer", ownerOrganization, "org", false, true, "me", domain.RoleWriter},
	{"organization reader", ownerOrganization, "org", false, false, "me", domain.RoleReader},
	{"own user board", ownerUser, "Me", false, true, "me", domain.RoleAdmin},
	{"other user board", ownerUser, "other", false, true, "me", domain.RoleWriter},
	{"unknown viewer", ownerUser, "me", false, false, "", domain.RoleReader},
}

func TestToRole(t *testing.T) {
	for _, tc := range roleCases {
		t.Run(tc.name, func(t *testing.T) {
			raw := &rawProject{ViewerCanUpdate: tc.canUpdate}
			raw.Owner.TypeName, raw.Owner.Login, raw.Owner.ViewerCanAdminister = tc.ownerType, tc.owner, tc.admin
			if got := toRole(raw, tc.viewer); got != tc.want {
				t.Fatalf("toRole = %s, want %s", got, tc.want)
			}
		})
	}
}

var (
	sampleText   = "x"
	emptyText    = ""
	sampleOption = "opt"
	sampleNumber = 2.5
	sampleDate   = time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
)

var fieldValueCases = []struct {
	name   string
	in     domain.ItemFieldValueInput
	member string
	fail   bool
}{
	{"text", domain.ItemFieldValueInput{Text: &sampleText}, "text", false},
	{"number", domain.ItemFieldValueInput{Number: &sampleNumber}, "number", false},
	{"date", domain.ItemFieldValueInput{Date: &sampleDate}, "date", false},
	{"option", domain.ItemFieldValueInput{SingleSelectID: &sampleOption}, "singleSelectOptionId", false},
	{"iteration", domain.ItemFieldValueInput{IterationID: &sampleOption}, "iterationId", false},
	{"empty text clears", domain.ItemFieldValueInput{Text: &emptyText}, "", true},
	{"nothing", domain.ItemFieldValueInput{}, "", true},
	{"two members", domain.ItemFieldValueInput{Text: &sampleText, Number: &sampleNumber}, "", true},
}

func TestFieldValueMembers(t *testing.T) {
	for _, tc := range fieldValueCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fieldValue(tc.in)
			if tc.fail {
				if !errors.Is(err, domain.ErrUsage) {
					t.Fatalf("expected ErrUsage, got %v", err)
				}
				return
			}
			assertSingleMember(t, got, err, tc.member)
		})
	}
}

func assertSingleMember(t *testing.T, got map[string]any, err error, member string) {
	t.Helper()
	if err != nil || len(got) != 1 || got[member] == nil {
		t.Fatalf("fieldValue = %v, %v", got, err)
	}
	if member == "date" && got["date"] != "2026-10-09" {
		t.Fatalf("date = %v", got["date"])
	}
}

var issueFieldMemberCases = []struct {
	name   string
	kind   string
	text   string
	member string
	value  any
	fail   error
}{
	{"text", issueFieldText, "hello", "textValue", "hello", nil},
	{"date", issueFieldDate, "2026-10-10", "dateValue", "2026-10-10", nil},
	{"bad date", issueFieldDate, "10/10/2026", "", nil, domain.ErrUsage},
	{"number", issueFieldNumber, "1.5", "numberValue", 1.5, nil},
	{"bad number", issueFieldNumber, "one", "", nil, domain.ErrUsage},
	{"option by name", issueFieldSingleSelect, "high", "singleSelectOptionId", "O2", nil},
	{"option by id", issueFieldSingleSelect, "O1", "singleSelectOptionId", "O1", nil},
	{"unknown option", issueFieldSingleSelect, "Hgh", "", nil, domain.ErrNotFound},
	{"multi select", issueFieldMultiSelect, "Urgent, high", "multiSelectOptionIds", []string{"O1", "O2"}, nil},
	{"multi select unknown", issueFieldMultiSelect, "Urgent, nope", "", nil, domain.ErrNotFound},
	{"unsupported", "GEO", "x", "", nil, domain.ErrUsage},
}

func TestIssueFieldMember(t *testing.T) {
	field := &rawIssueField{Name: "Priority", Options: []rawOption{{ID: "O1", Name: "Urgent"}, {ID: "O2", Name: "High"}}}
	for _, tc := range issueFieldMemberCases {
		t.Run(tc.name, func(t *testing.T) {
			field.DataType = tc.kind
			got, err := issueFieldMember(field, tc.text)
			if tc.fail != nil {
				if !errors.Is(err, tc.fail) {
					t.Fatalf("expected %v, got %v", tc.fail, err)
				}
				return
			}
			if err != nil || got.name != tc.member {
				t.Fatalf("member = %+v, %v", got, err)
			}
			assertMemberValue(t, got.value, tc.value)
		})
	}
}

func assertMemberValue(t *testing.T, got, want any) {
	t.Helper()
	ids, ok := want.([]string)
	if !ok {
		if got != want {
			t.Fatalf("value = %v, want %v", got, want)
		}
		return
	}
	gotIDs, _ := got.([]string)
	if len(gotIDs) != len(ids) || gotIDs[0] != ids[0] || gotIDs[1] != ids[1] {
		t.Fatalf("ids = %v, want %v", gotIDs, ids)
	}
}

func TestSmallHelpers(t *testing.T) {
	if pageSize(0) != itemPageSize || pageSize(5) != 5 || pageSize(500) != itemPageSize {
		t.Fatal("pageSize")
	}
	n := 0.5
	if formatNumber(nil) != "" || formatNumber(&n) != "0.5" {
		t.Fatal("formatNumber")
	}
	if (rawIssueFieldValue{TypeName: "Other"}).text() != "" {
		t.Fatal("unknown issue field value kinds read as empty")
	}
	named := rawFieldValue{TypeName: valueIssueField}
	named.Field.Name = "x"
	if _, ok := toFieldValue(named); ok {
		t.Fatal("an issue field value without payload is skipped")
	}
	if _, ok := toItem(rawItem{Content: &rawIssue{TypeName: "DraftIssue"}}); ok {
		t.Fatal("drafts are skipped")
	}
	calls := 0
	err := forEachPage(func(string) (pageInfo, error) {
		calls++
		return pageInfo{HasNextPage: true, EndCursor: "same"}, nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("a repeated cursor must stop the loop after the second page, got %d calls", calls)
	}
	if toIterations(nil) != nil {
		t.Fatal("no configuration means no iterations")
	}
}
