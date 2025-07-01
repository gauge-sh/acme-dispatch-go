package store

import (
	"errors"
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

func TestInactiveTechnicianRules(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	dana := seedTech(t, s, "Dana")
	existing := seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))

	dana.IsActive = false
	if err := s.UpdateTechnician(dana); err != nil {
		t.Fatal(err)
	}

	a := core.Appointment{JobID: job.ID, TechnicianID: dana.ID, ScheduledStart: ts(3, 9), ScheduledEnd: ts(3, 10)}
	err := s.CreateAppointment(&a)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "technician is not active" {
		t.Fatalf("inactive assignment: %v", err)
	}

	// Existing appointments still take notes and reschedules.
	existing.CompletionNotes = str("wrapped up before leaving")
	if err := s.UpdateAppointment(existing); err != nil {
		t.Fatalf("notes on inactive tech: %v", err)
	}

	active := true
	_, total, _ := s.ListTechnicians(TechnicianFilter{Active: &active}, Page{})
	if total != 0 {
		t.Fatalf("active filter: %d", total)
	}
	inactive := false
	_, total, _ = s.ListTechnicians(TechnicianFilter{Active: &inactive}, Page{})
	if total != 1 {
		t.Fatalf("inactive filter: %d", total)
	}
}
