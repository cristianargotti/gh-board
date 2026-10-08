package domain_test

import (
	"testing"

	"github.com/cristianargotti/gh-board/internal/domain"
)

func TestSnapshotOmitted(t *testing.T) {
	s := domain.Snapshot{Completeness: domain.Complete}
	s.MarkOmitted(domain.SectionMine, 0)
	if s.Completeness != domain.Complete || s.OmittedTotal() != 0 {
		t.Fatal("marking zero must change nothing")
	}
	s.MarkOmitted(domain.SectionMine, 2)
	s.MarkOmitted(domain.SectionMine, 1)
	s.MarkOmitted(domain.SectionEpics, 4)
	if s.Completeness != domain.Partial {
		t.Fatal("omissions make the snapshot partial")
	}
	if s.Omitted[domain.SectionMine] != 3 || s.OmittedTotal() != 7 {
		t.Fatalf("Omitted = %v", s.Omitted)
	}
}

func TestSnapshotSectionsOrder(t *testing.T) {
	want := []string{"rules", "sprint", "mine", "attention", "triage", "epics", "deliveries"}
	if len(domain.SnapshotSections) != len(want) {
		t.Fatalf("SnapshotSections = %v", domain.SnapshotSections)
	}
	for i, name := range want {
		if domain.SnapshotSections[i] != name {
			t.Fatalf("section %d = %q, want %q", i, domain.SnapshotSections[i], name)
		}
	}
}
