package core

import "fmt"

var nextStatuses = map[JobStatus][]JobStatus{
	StatusRequested:  {StatusScheduled, StatusCancelled},
	StatusScheduled:  {StatusInProgress, StatusCancelled},
	StatusInProgress: {StatusCompleted, StatusCancelled},
	StatusCompleted:  {StatusInvoiced},
	StatusInvoiced:   {},
	StatusCancelled:  {},
}

func CanTransition(from, to JobStatus) bool {
	for _, next := range nextStatuses[from] {
		if next == to {
			return true
		}
	}
	return false
}

func IsTerminal(status JobStatus) bool {
	return status == StatusInvoiced || status == StatusCancelled
}

// CheckTransition guards the generic transition path. invoiced is part of the
// machine but is never reachable here: creating the invoice record is what
// moves a completed job, so asking for it directly is always refused.
func CheckTransition(from, to JobStatus, appointmentCount int) error {
	if to == StatusInvoiced || !CanTransition(from, to) {
		return InvalidTransition(fmt.Sprintf("cannot move a job from %s to %s", from, to))
	}
	if to == StatusScheduled && appointmentCount < 1 {
		return Invalid("a job needs an appointment before it can be scheduled")
	}
	return nil
}

func acceptsAppointments(status JobStatus) bool {
	switch status {
	case StatusRequested, StatusScheduled, StatusInProgress:
		return true
	}
	return false
}

func CheckAcceptsAppointments(status JobStatus) error {
	if !acceptsAppointments(status) {
		return Invalid(fmt.Sprintf("appointments cannot be added to a job that is %s", status))
	}
	return nil
}

func CheckAcceptsParts(status JobStatus) error {
	if IsTerminal(status) {
		return Invalid(fmt.Sprintf("parts cannot be added to a job that is %s", status))
	}
	return nil
}
