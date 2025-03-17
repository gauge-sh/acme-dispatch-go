package core

import "time"

// Booking is the slice of an appointment the conflict rule cares about.
// ID is zero for an appointment that has not been saved yet.
type Booking struct {
	ID           int64
	TechnicianID int64
	Start        time.Time
	End          time.Time
}

func CheckWindow(start, end time.Time) error {
	if !end.After(start) {
		return Invalid("appointment must end after it starts")
	}
	return nil
}

// Overlaps is half-open [start, end): 09:00–10:00 and 10:00–11:00 sit next
// to each other without conflicting.
func Overlaps(a, b Booking) bool {
	return a.Start.Before(b.End) && b.Start.Before(a.End)
}

// FindConflict reports the first existing booking the candidate collides
// with. A saved appointment being rescheduled appears in existing; it is not
// in the way of itself.
func FindConflict(candidate Booking, existing []Booking) (Booking, bool) {
	for _, booked := range existing {
		if booked.TechnicianID != candidate.TechnicianID {
			continue
		}
		if candidate.ID != 0 && booked.ID == candidate.ID {
			continue
		}
		if Overlaps(candidate, booked) {
			return booked, true
		}
	}
	return Booking{}, false
}

func CheckAvailability(candidate Booking, existing []Booking) error {
	if err := CheckWindow(candidate.Start, candidate.End); err != nil {
		return err
	}
	if _, found := FindConflict(candidate, existing); found {
		return ScheduleConflict("technician is already booked for that time")
	}
	return nil
}

// IsWeekend is Saturday or Sunday in UTC, whatever zone the caller had.
func IsWeekend(moment time.Time) bool {
	day := moment.UTC().Weekday()
	return day == time.Saturday || day == time.Sunday
}
