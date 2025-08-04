package server

import (
	"net/http"
	"testing"
)

func TestAppointmentConflictOverAPI(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana", 8000)
	job := api.createJob(customer["id"], "Fix furnace", "")
	other := api.createJob(customer["id"], "Water heater", "")

	api.createAppointment(job["id"], tech["id"], "2025-03-12T09:00:00Z", "2025-03-12T11:00:00Z")

	rec := api.do("POST", "/api/v1/appointments", api.staff, map[string]any{
		"job_id": other["id"], "technician_id": tech["id"],
		"scheduled_start": "2025-03-12T10:00:00Z", "scheduled_end": "2025-03-12T12:00:00Z",
	})
	api.mustError(rec, http.StatusConflict, "schedule_conflict", "technician is already booked for that time")

	// Touching windows are fine: [start, end) is half-open.
	api.createAppointment(other["id"], tech["id"], "2025-03-12T11:00:00Z", "2025-03-12T12:00:00Z")
}

func TestAppointmentRescheduleOverAPI(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana", 8000)
	job := api.createJob(customer["id"], "Fix furnace", "")

	appt := api.createAppointment(job["id"], tech["id"], "2025-03-12T09:00:00Z", "2025-03-12T11:00:00Z")
	blocker := api.createAppointment(job["id"], tech["id"], "2025-03-12T13:00:00Z", "2025-03-12T14:00:00Z")

	// Sliding within its own old window: an appointment never conflicts
	// with itself.
	rec := api.do("PATCH", jsonPath("/api/v1/appointments/%v", appt["id"]), api.staff, map[string]any{
		"scheduled_start": "2025-03-12T10:00:00Z", "scheduled_end": "2025-03-12T12:00:00Z",
	})
	api.mustStatus(rec, http.StatusOK)

	rec = api.do("PATCH", jsonPath("/api/v1/appointments/%v", appt["id"]), api.staff, map[string]any{
		"scheduled_start": "2025-03-12T13:30:00Z", "scheduled_end": "2025-03-12T14:30:00Z",
	})
	api.mustError(rec, http.StatusConflict, "schedule_conflict", "")
	_ = blocker

	rec = api.do("DELETE", jsonPath("/api/v1/appointments/%v", blocker["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusNoContent)
	rec = api.do("PATCH", jsonPath("/api/v1/appointments/%v", appt["id"]), api.staff, map[string]any{
		"scheduled_start": "2025-03-12T13:30:00Z", "scheduled_end": "2025-03-12T14:30:00Z",
	})
	api.mustStatus(rec, http.StatusOK)
}

func TestAppointmentValidationOverAPI(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana", 8000)
	job := api.createJob(customer["id"], "Fix furnace", "")

	rec := api.do("POST", "/api/v1/appointments", api.staff, map[string]any{
		"job_id": job["id"], "technician_id": tech["id"],
		"scheduled_start": "2025-03-12T11:00:00Z", "scheduled_end": "2025-03-12T11:00:00Z",
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "appointment must end after it starts")

	rec = api.do("POST", "/api/v1/appointments", api.staff, map[string]any{
		"job_id": job["id"], "technician_id": tech["id"],
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "")

	rec = api.do("POST", "/api/v1/appointments", api.staff, map[string]any{
		"job_id": 999, "technician_id": tech["id"],
		"scheduled_start": "2025-03-12T09:00:00Z", "scheduled_end": "2025-03-12T10:00:00Z",
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "job_id must exist")

	// Cancelled jobs stop taking appointments.
	api.createAppointment(job["id"], tech["id"], "2025-03-12T09:00:00Z", "2025-03-12T10:00:00Z")
	api.mustStatus(api.transition(job["id"], "scheduled"), http.StatusOK)
	api.mustStatus(api.transition(job["id"], "cancelled"), http.StatusOK)
	rec = api.do("POST", "/api/v1/appointments", api.staff, map[string]any{
		"job_id": job["id"], "technician_id": tech["id"],
		"scheduled_start": "2025-03-13T09:00:00Z", "scheduled_end": "2025-03-13T10:00:00Z",
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "appointments cannot be added to a job that is cancelled")
}

func TestAppointmentListFilters(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	dana := api.createTechnician("Dana", 8000)
	marcus := api.createTechnician("Marcus", 6000)
	job := api.createJob(customer["id"], "Fix furnace", "")

	api.createAppointment(job["id"], dana["id"], "2025-03-12T09:00:00Z", "2025-03-12T10:00:00Z")
	api.createAppointment(job["id"], marcus["id"], "2025-03-14T09:00:00Z", "2025-03-14T10:00:00Z")
	// Created out of chronological order to prove the sort.
	api.createAppointment(job["id"], dana["id"], "2025-03-10T09:00:00Z", "2025-03-10T10:00:00Z")

	rec := api.do("GET", "/api/v1/appointments", api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Total != 3 || list.Items[0]["scheduled_start"] != "2025-03-10T09:00:00Z" {
		t.Fatalf("ordering: %+v", list.Items)
	}

	rec = api.do("GET", jsonPath("/api/v1/appointments?technician_id=%v", dana["id"]), api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 2 {
		t.Fatalf("technician filter: %+v", list)
	}

	rec = api.do("GET", "/api/v1/appointments?from=2025-03-11T00:00:00Z&to=2025-03-13T00:00:00Z", api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 1 {
		t.Fatalf("range filter: %+v", list)
	}

	rec = api.do("GET", "/api/v1/appointments?from=yesterday", api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "from must be an ISO-8601 timestamp")
}
