package core

import (
	"errors"
	"testing"
)

var legalMoves = map[JobStatus][]JobStatus{
	StatusRequested:  {StatusScheduled, StatusCancelled},
	StatusScheduled:  {StatusInProgress, StatusCancelled},
	StatusInProgress: {StatusCompleted, StatusCancelled},
	StatusCompleted:  {StatusInvoiced},
}

func TestEveryLegalAndIllegalTransition(t *testing.T) {
	for _, from := range JobStatuses {
		for _, to := range JobStatuses {
			legal := false
			for _, next := range legalMoves[from] {
				if next == to {
					legal = true
				}
			}
			if got := CanTransition(from, to); got != legal {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, legal)
			}
		}
	}
}

func TestCheckTransitionRefusesInvoiced(t *testing.T) {
	// completed -> invoiced is a real edge, but only invoicing may walk it.
	err := CheckTransition(StatusCompleted, StatusInvoiced, 1)
	var derr *Error
	if !errors.As(err, &derr) || derr.Code != CodeInvalidTransition {
		t.Fatalf("expected invalid_transition, got %v", err)
	}
	if derr.Message != "cannot move a job from completed to invoiced" {
		t.Fatalf("unexpected message %q", derr.Message)
	}
}

func TestCheckTransitionMessages(t *testing.T) {
	err := CheckTransition(StatusInvoiced, StatusCancelled, 0)
	var derr *Error
	if !errors.As(err, &derr) || derr.Code != CodeInvalidTransition {
		t.Fatalf("expected invalid_transition, got %v", err)
	}
	if derr.Message != "cannot move a job from invoiced to cancelled" {
		t.Fatalf("unexpected message %q", derr.Message)
	}
}

func TestSchedulingNeedsAnAppointment(t *testing.T) {
	err := CheckTransition(StatusRequested, StatusScheduled, 0)
	var derr *Error
	if !errors.As(err, &derr) || derr.Code != CodeValidationFailed {
		t.Fatalf("expected validation_failed, got %v", err)
	}
	if derr.Message != "a job needs an appointment before it can be scheduled" {
		t.Fatalf("unexpected message %q", derr.Message)
	}
	if err := CheckTransition(StatusRequested, StatusScheduled, 1); err != nil {
		t.Fatalf("one appointment should be enough: %v", err)
	}
}

func TestCancelLegalFromPreCompletedOnly(t *testing.T) {
	for _, from := range []JobStatus{StatusRequested, StatusScheduled, StatusInProgress} {
		if err := CheckTransition(from, StatusCancelled, 0); err != nil {
			t.Errorf("cancel from %s: %v", from, err)
		}
	}
	for _, from := range []JobStatus{StatusCompleted, StatusInvoiced, StatusCancelled} {
		if err := CheckTransition(from, StatusCancelled, 0); err == nil {
			t.Errorf("cancel from %s should fail", from)
		}
	}
}

func TestAppointmentAndPartGates(t *testing.T) {
	for _, st := range []JobStatus{StatusRequested, StatusScheduled, StatusInProgress} {
		if err := CheckAcceptsAppointments(st); err != nil {
			t.Errorf("appointments on %s: %v", st, err)
		}
		if err := CheckAcceptsParts(st); err != nil {
			t.Errorf("parts on %s: %v", st, err)
		}
	}
	// completed jobs still take parts (billing happens at invoice time) but
	// no longer take appointments.
	if err := CheckAcceptsParts(StatusCompleted); err != nil {
		t.Errorf("parts on completed: %v", err)
	}
	var derr *Error
	err := CheckAcceptsAppointments(StatusCompleted)
	if !errors.As(err, &derr) || derr.Message != "appointments cannot be added to a job that is completed" {
		t.Errorf("appointments on completed: %v", err)
	}
	for _, st := range []JobStatus{StatusInvoiced, StatusCancelled} {
		err := CheckAcceptsAppointments(st)
		if !errors.As(err, &derr) || derr.Code != CodeValidationFailed {
			t.Errorf("appointments on %s: %v", st, err)
		}
		err = CheckAcceptsParts(st)
		if !errors.As(err, &derr) || derr.Message != "parts cannot be added to a job that is "+string(st) {
			t.Errorf("parts on %s: %v", st, err)
		}
	}
}
