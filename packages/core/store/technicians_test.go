package store

import (
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func seedTech(t *testing.T, s *Store, name string, tags ...string) *core.Technician {
	t.Helper()
	tech := core.Technician{Name: name, HourlyCostCents: 5000, SkillTags: core.StringList(tags), IsActive: true}
	if err := s.CreateTechnician(&tech); err != nil {
		t.Fatal(err)
	}
	return &tech
}

func TestTechnicianRoundTrip(t *testing.T) {
	s := testStore(t)
	tech := seedTech(t, s, "Dana Reyes", "HVAC", "electrical", "hvac")

	got, err := s.TechnicianByID(tech.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SkillTags) != 2 || got.SkillTags[0] != "electrical" || got.SkillTags[1] != "hvac" {
		t.Fatalf("tags not normalized on the way in: %v", got.SkillTags)
	}

	got.HourlyCostCents = 6500
	if err := s.UpdateTechnician(got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.TechnicianByID(tech.ID)
	if got.HourlyCostCents != 6500 {
		t.Fatalf("rate update lost: %+v", got)
	}
}

func TestTechnicianSkillFilter(t *testing.T) {
	s := testStore(t)
	seedTech(t, s, "Dana", "hvac", "plumbing")
	seedTech(t, s, "Marcus", "electrical")
	seedTech(t, s, "Iris", "plumbing")

	items, total, err := s.ListTechnicians(TechnicianFilter{Skill: "plumbing"}, Page{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("plumbing filter: total=%d", total)
	}
	// Exact match only — no substring creep.
	_, total, _ = s.ListTechnicians(TechnicianFilter{Skill: "plumb"}, Page{})
	if total != 0 {
		t.Fatalf("partial tag matched: total=%d", total)
	}
}
