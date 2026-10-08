package domain_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

var renamedStatusConfig = &domain.Config{Capabilities: domain.Capabilities{
	Status: &domain.StatusCapability{Field: "Estado", Active: []string{"Fazendo"}, Done: []string{"Feito"}},
}}

var statusFieldNameCases = []struct {
	name string
	cfg  *domain.Config
	want string
}{
	{"generic", nil, domain.BuiltinStatusField},
	{"no status capability", &domain.Config{}, domain.BuiltinStatusField},
	{"configured", renamedStatusConfig, "Estado"},
}

func TestStatusFieldName(t *testing.T) {
	for _, tc := range statusFieldNameCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.StatusFieldName(tc.cfg); got != tc.want {
				t.Fatalf("StatusFieldName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStatusField(t *testing.T) {
	field, ok := domain.StatusField(fixtureConfig(), fixtureProject())
	if !ok || field.ID != "F_status" {
		t.Fatalf("StatusField = %+v, %v", field, ok)
	}
	if _, ok := domain.StatusField(renamedStatusConfig, fixtureProject()); ok {
		t.Fatal("a field absent from the project must not resolve")
	}
	text := domain.Project{Fields: []domain.Field{{Name: fxStatus, DataType: domain.DataTypeText}}}
	if _, ok := domain.StatusField(nil, text); ok {
		t.Fatal("a status field that is not single select must not resolve")
	}
}

var itemClassCases = []struct {
	name   string
	cfg    *domain.Config
	item   domain.Item
	class  domain.StatusClass
	active bool
	done   bool
}{
	{"active option", fixtureConfig(), newItem(1, "a", withStatus(stActive, fxNow)), domain.StatusActive, true, false},
	{"test option is active", fixtureConfig(), newItem(1, "a", withStatus(stTest, fxNow)), domain.StatusActive, true, false},
	{"done option", fixtureConfig(), newItem(1, "a", withStatus(stDone, fxNow)), domain.StatusDone, false, true},
	{"ready option", fixtureConfig(), newItem(1, "a", withStatus(stReady, fxNow)), domain.StatusReady, false, false},
	{"unmapped open option", fixtureConfig(), newItem(1, "a", withStatus("ARCHIVED", fxNow)), domain.StatusUnknown, false, false},
	{"unmapped closed option", fixtureConfig(), newItem(1, "a", withStatus("ARCHIVED", fxNow), withClosed(fxNow)), domain.StatusUnknown, false, true},
	{"no value", fixtureConfig(), newItem(1, "a"), domain.StatusUnknown, false, false},
	{"generic open", nil, newItem(1, "a", withStatus(stDone, fxNow)), domain.StatusUnknown, false, false},
	{"generic closed", nil, newItem(1, "a", withClosed(fxNow)), domain.StatusUnknown, false, true},
}

func TestItemStatusClasses(t *testing.T) {
	for _, tc := range itemClassCases {
		t.Run(tc.name, func(t *testing.T) {
			value, class := domain.ItemStatus(tc.cfg, tc.item)
			if class != tc.class || value != tc.item.Text(fxStatus) {
				t.Fatalf("ItemStatus = %q, %v, want class %v", value, class, tc.class)
			}
			if got := domain.ClassifyStatus(tc.cfg, value); got != tc.class {
				t.Fatalf("ClassifyStatus = %v, want %v", got, tc.class)
			}
			if domain.IsActive(tc.cfg, tc.item) != tc.active || domain.IsDone(tc.cfg, tc.item) != tc.done {
				t.Fatalf("IsActive = %v, IsDone = %v", domain.IsActive(tc.cfg, tc.item), domain.IsDone(tc.cfg, tc.item))
			}
		})
	}
}

func TestWorkableAndPending(t *testing.T) {
	items := []domain.Item{
		newItem(1, "open active", withStatus(stActive, fxNow)),
		newItem(2, "closed", withClosed(fxNow)),
		newItem(3, "archived", archived()),
		newItem(4, "open done", withStatus(stDone, fxNow)),
		newItem(5, "open backlog", withStatus(stBacklog, fxNow)),
	}
	workable := domain.Workable(items)
	if len(workable) != 3 || workable[0].Issue.Number != 1 || workable[1].Issue.Number != 4 || workable[2].Issue.Number != 5 {
		t.Fatalf("Workable = %v", numbers(workable))
	}
	pending := domain.Pending(fixtureConfig(), items)
	if len(pending) != 2 || pending[0].Issue.Number != 1 || pending[1].Issue.Number != 5 {
		t.Fatalf("Pending = %v", numbers(pending))
	}
	if generic := domain.Pending(nil, items); len(generic) != 3 {
		t.Fatalf("generic Pending = %v", numbers(generic))
	}
}

func TestGenericCapabilities(t *testing.T) {
	var cfg *domain.Config
	if !cfg.Generic() || fixtureConfig().Generic() {
		t.Fatal("Generic must be true only for a nil configuration")
	}
	if caps := domain.CapabilitiesOf(nil); caps != (domain.Capabilities{}) {
		t.Fatalf("CapabilitiesOf(nil) = %+v", caps)
	}
	if caps := domain.CapabilitiesOf(fixtureConfig()); caps.Status == nil || caps.Triage == nil {
		t.Fatal("CapabilitiesOf must return the declared capabilities")
	}
}

func numbers(items []domain.Item) []int {
	out := make([]int, 0, len(items))
	for _, it := range items {
		out = append(out, it.Issue.Number)
	}
	return out
}
