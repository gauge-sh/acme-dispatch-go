package core

import (
	"errors"
	"testing"
	"time"
)

func at(hour, min int) time.Time {
	return time.Date(2025, 3, 12, hour, min, 0, 0, time.UTC)
}

func TestOverlapIsHalfOpen(t *testing.T) {
	base := Booking{ID: 1, TechnicianID: 7, Start: at(9, 0), End: at(10, 0)}
	cases := []struct {
		name       string
		start, end time.Time
		want       bool
	}{
		{"identical window", at(9, 0), at(10, 0), true},
		{"contained", at(9, 15), at(9, 45), true},
		{"straddles start", at(8, 30), at(9, 30), true},
		{"straddles end", at(9, 30), at(10, 30), true},
		{"surrounds", at(8, 0), at(11, 0), true},
		{"touches end", at(10, 0), at(11, 0), false},
		{"touches start", at(8, 0), at(9, 0), false},
		{"clear before", at(7, 0), at(8, 0), false},
		{"clear after", at(11, 0), at(12, 0), false},
	}
	for _, c := range cases {
		got := Overlaps(Booking{TechnicianID: 7, Start: c.start, End: c.end}, base)
		if got != c.want {
			t.Errorf("%s: Overlaps = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestConflictIgnoresOtherTechnicians(t *testing.T) {
	existing := []Booking{{ID: 1, TechnicianID: 2, Start: at(9, 0), End: at(10, 0)}}
	candidate := Booking{TechnicianID: 3, Start: at(9, 0), End: at(10, 0)}
	if _, found := FindConflict(candidate, existing); found {
		t.Fatal("different technicians never conflict")
	}
}

func TestRescheduleDoesNotConflictWithItself(t *testing.T) {
	existing := []Booking{
		{ID: 5, TechnicianID: 2, Start: at(9, 0), End: at(10, 0)},
		{ID: 6, TechnicianID: 2, Start: at(13, 0), End: at(14, 0)},
	}
	// Nudging appointment 5 within its own old window is fine...
	moved := Booking{ID: 5, TechnicianID: 2, Start: at(9, 30), End: at(10, 30)}
	if err := CheckAvailability(moved, existing); err != nil {
		t.Fatalf("reschedule collided with itself: %v", err)
	}
	// ...but not into a colleague booking.
	moved = Booking{ID: 5, TechnicianID: 2, Start: at(13, 30), End: at(14, 30)}
	err := CheckAvailability(moved, existing)
	var derr *Error
	if !errors.As(err, &derr) || derr.Code != CodeScheduleConflict {
		t.Fatalf("expected schedule_conflict, got %v", err)
	}
	if derr.Message != "technician is already booked for that time" {
		t.Fatalf("unexpected message %q", derr.Message)
	}
}

func TestCheckWindow(t *testing.T) {
	var derr *Error
	err := CheckWindow(at(10, 0), at(10, 0))
	if !errors.As(err, &derr) || derr.Message != "appointment must end after it starts" {
		t.Fatalf("zero-length window: %v", err)
	}
	if err := CheckWindow(at(10, 0), at(9, 0)); err == nil {
		t.Fatal("backwards window accepted")
	}
	if err := CheckWindow(at(10, 0), at(10, 1)); err != nil {
		t.Fatalf("one-minute window rejected: %v", err)
	}
}

func TestIsWeekendUsesUTC(t *testing.T) {
	saturday := time.Date(2025, 3, 15, 9, 0, 0, 0, time.UTC)
	if !IsWeekend(saturday) {
		t.Error("saturday should be a weekend")
	}
	if !IsWeekend(time.Date(2025, 3, 16, 23, 0, 0, 0, time.UTC)) {
		t.Error("sunday should be a weekend")
	}
	if IsWeekend(time.Date(2025, 3, 12, 9, 0, 0, 0, time.UTC)) {
		t.Error("wednesday is not a weekend")
	}
	// Friday 20:00 in New York is already Saturday in UTC.
	ny, err := time.LoadLocation("America/New_York")
	if err == nil {
		fridayEvening := time.Date(2025, 3, 14, 20, 0, 0, 0, ny)
		if !IsWeekend(fridayEvening) {
			t.Error("weekend is decided in UTC, not the wall clock")
		}
	}
}
