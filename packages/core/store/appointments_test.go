package store

import (
	"errors"
	"testing"
	"time"

	"github.com/acme-dispatch/dispatch/core"
)

func ts(day, hour int) core.Time {
	return core.NewTime(time.Date(2025, 6, day, hour, 0, 0, 0, time.UTC))
}

func seedAppointment(t *testing.T, s *Store, jobID, techID int64, start, end core.Time) *core.Appointment {
	t.Helper()
	a := core.Appointment{JobID: jobID, TechnicianID: techID, ScheduledStart: start, ScheduledEnd: end}
	if err := s.CreateAppointment(&a); err != nil {
		t.Fatal(err)
	}
	return &a
}

func TestAppointmentConflicts(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	other := seedJob(t, s, c.ID, "Water heater")
	dana := seedTech(t, s, "Dana")
	marcus := seedTech(t, s, "Marcus")

	seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))

	// Same technician, overlapping window, even on another job: 409.
	clash := core.Appointment{JobID: other.ID, TechnicianID: dana.ID, ScheduledStart: ts(2, 10), ScheduledEnd: ts(2, 12)}
	err := s.CreateAppointment(&clash)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeScheduleConflict {
		t.Fatalf("expected schedule_conflict, got %v", err)
	}

	// Touching windows do not overlap.
	if _, err := s.AppointmentByID(seedAppointment(t, s, other.ID, dana.ID, ts(2, 11), ts(2, 12)).ID); err != nil {
		t.Fatalf("touching window rejected: %v", err)
	}

	// A different technician is free at the same time.
	seedAppointment(t, s, other.ID, marcus.ID, ts(2, 10), ts(2, 12))
}

func TestAppointmentRescheduleSkipsItself(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	dana := seedTech(t, s, "Dana")
	appt := seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))
	blocker := seedAppointment(t, s, job.ID, dana.ID, ts(2, 13), ts(2, 14))

	appt.ScheduledStart, appt.ScheduledEnd = ts(2, 10), ts(2, 12)
	if err := s.UpdateAppointment(appt); err != nil {
		t.Fatalf("reschedule within own slot: %v", err)
	}

	appt.ScheduledStart, appt.ScheduledEnd = ts(2, 13), ts(2, 15)
	err := s.UpdateAppointment(appt)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeScheduleConflict {
		t.Fatalf("expected conflict with %d, got %v", blocker.ID, err)
	}
}

func TestAppointmentJobGates(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	dana := seedTech(t, s, "Dana")
	appt := seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))

	for _, target := range []core.JobStatus{core.StatusScheduled, core.StatusInProgress, core.StatusCompleted} {
		if _, err := s.TransitionJob(job.ID, target); err != nil {
			t.Fatal(err)
		}
	}

	// Completed jobs stop accepting appointments...
	late := core.Appointment{JobID: job.ID, TechnicianID: dana.ID, ScheduledStart: ts(3, 9), ScheduledEnd: ts(3, 10)}
	err := s.CreateAppointment(&late)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "appointments cannot be added to a job that is completed" {
		t.Fatalf("gate: %v", err)
	}

	// ...but completion notes on the existing one still save.
	appt.CompletionNotes = str("replaced igniter, tested twice")
	if err := s.UpdateAppointment(appt); err != nil {
		t.Fatalf("notes after completion: %v", err)
	}
}

func TestTransitionNeedsAppointment(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")

	_, err := s.TransitionJob(job.ID, core.StatusScheduled)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "a job needs an appointment before it can be scheduled" {
		t.Fatalf("expected appointment requirement, got %v", err)
	}

	dana := seedTech(t, s, "Dana")
	seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))
	if _, err := s.TransitionJob(job.ID, core.StatusScheduled); err != nil {
		t.Fatal(err)
	}

	// cancelled is terminal: no way back.
	if _, err := s.TransitionJob(job.ID, core.StatusCancelled); err != nil {
		t.Fatal(err)
	}
	_, err = s.TransitionJob(job.ID, core.StatusInProgress)
	if !errors.As(err, &derr) || derr.Code != core.CodeInvalidTransition {
		t.Fatalf("cancelled should be terminal: %v", err)
	}
}

func TestDeleteTechnicianWithAppointments(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	dana := seedTech(t, s, "Dana")
	seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))

	err := s.DeleteTechnician(dana.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "technician has appointments and cannot be deleted" {
		t.Fatalf("expected guard, got %v", err)
	}
}

func TestAppointmentWindowAndRangeFilter(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	dana := seedTech(t, s, "Dana")

	bad := core.Appointment{JobID: job.ID, TechnicianID: dana.ID, ScheduledStart: ts(2, 11), ScheduledEnd: ts(2, 11)}
	err := s.CreateAppointment(&bad)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "appointment must end after it starts" {
		t.Fatalf("zero-length window: %v", err)
	}

	seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 10))
	seedAppointment(t, s, job.ID, dana.ID, ts(4, 9), ts(4, 10))
	from, to := ts(3, 0), ts(5, 0)
	_, total, err := s.ListAppointments(AppointmentFilter{From: &from, To: &to}, Page{})
	if err != nil || total != 1 {
		t.Fatalf("range filter: total=%d err=%v", total, err)
	}
}
