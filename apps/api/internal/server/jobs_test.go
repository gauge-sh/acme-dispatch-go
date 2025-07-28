package server

import (
	"net/http"
	"testing"
)

func TestJobStatusIsNeverWritable(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")

	rec := api.do("POST", "/api/v1/jobs", api.staff, map[string]any{
		"customer_id": customer["id"],
		"title":       "Fix furnace",
		"status":      "completed", // ignored, not an error
	})
	api.mustStatus(rec, http.StatusCreated)
	job := decodeBody[map[string]any](t, rec)
	if job["status"] != "requested" {
		t.Fatalf("status: %v", job["status"])
	}

	rec = api.do("PATCH", jsonPath("/api/v1/jobs/%v", job["id"]), api.staff, map[string]any{
		"title":  "Fix furnace today",
		"status": "invoiced",
	})
	api.mustStatus(rec, http.StatusOK)
	patched := decodeBody[map[string]any](t, rec)
	if patched["status"] != "requested" || patched["title"] != "Fix furnace today" {
		t.Fatalf("patch: %v", patched)
	}
}

func TestJobDetailNestsRelations(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana", 8000)
	part := api.createPart("IGN-1", 1299, 10)
	job := api.createJob(customer["id"], "Fix furnace", "urgent")

	api.createAppointment(job["id"], tech["id"], "2025-03-12T13:00:00Z", "2025-03-12T14:30:00Z")
	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 3,
	})
	api.mustStatus(rec, http.StatusCreated)

	rec = api.do("GET", jsonPath("/api/v1/jobs/%v", job["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusOK)
	detail := decodeBody[map[string]any](t, rec)
	appointments := detail["appointments"].([]any)
	jobParts := detail["job_parts"].([]any)
	if len(appointments) != 1 || len(jobParts) != 1 {
		t.Fatalf("nesting: %v", detail)
	}
	appt := appointments[0].(map[string]any)
	if appt["scheduled_start"] != "2025-03-12T13:00:00Z" {
		t.Fatalf("timestamp format: %v", appt["scheduled_start"])
	}
	jp := jobParts[0].(map[string]any)
	if jp["unit_price_cents"] != float64(1299) {
		t.Fatalf("captured price: %v", jp)
	}
	if invoiceID, present := detail["invoice_record_id"]; !present || invoiceID != nil {
		t.Fatalf("invoice_record_id should be explicit null: %v", detail)
	}

	// The list route stays flat.
	rec = api.do("GET", "/api/v1/jobs", api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if _, nested := list.Items[0]["appointments"]; nested {
		t.Fatal("list route must not nest")
	}
}

func TestTransitionEndpoint(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana", 8000)
	job := api.createJob(customer["id"], "Fix furnace", "")

	rec := api.transition(job["id"], "sideways")
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "unknown status")

	// invoiced is a real enum value but never legal here.
	rec = api.transition(job["id"], "invoiced")
	api.mustError(rec, http.StatusConflict, "invalid_transition", "")

	rec = api.transition(job["id"], "scheduled")
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "a job needs an appointment before it can be scheduled")

	api.createAppointment(job["id"], tech["id"], "2025-03-12T09:00:00Z", "2025-03-12T11:00:00Z")
	rec = api.transition(job["id"], "scheduled")
	api.mustStatus(rec, http.StatusOK)
	moved := decodeBody[map[string]any](t, rec)
	if moved["status"] != "scheduled" {
		t.Fatalf("transition result: %v", moved)
	}

	rec = api.transition(job["id"], "completed")
	body := api.mustError(rec, http.StatusConflict, "invalid_transition", "cannot move a job from scheduled to completed")
	_ = body

	rec = api.transition(job["id"], "cancelled")
	api.mustStatus(rec, http.StatusOK)
	rec = api.transition(job["id"], "in_progress")
	api.mustError(rec, http.StatusConflict, "invalid_transition", "cannot move a job from cancelled to in_progress")
}

func TestJobFiltersAndSearch(t *testing.T) {
	api := newTestAPI(t)
	c1 := api.createCustomer("Harbor Coffee")
	c2 := api.createCustomer("Northside Dental")
	api.createJob(c1["id"], "Furnace not igniting", "urgent")
	api.createJob(c1["id"], "Water heater leak", "")
	api.createJob(c2["id"], "Compressor rattle", "low")

	rec := api.do("GET", jsonPath("/api/v1/jobs?customer_id=%v", c1["id"]), api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 2 {
		t.Fatalf("customer filter: %+v", list)
	}
	rec = api.do("GET", "/api/v1/jobs?priority=urgent", api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 1 {
		t.Fatalf("priority filter: %+v", list)
	}
	rec = api.do("GET", "/api/v1/jobs?q=heater", api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 1 {
		t.Fatalf("title filter: %+v", list)
	}
	rec = api.do("GET", "/api/v1/jobs?status=requested,cancelled", api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 3 {
		t.Fatalf("status list filter: %+v", list)
	}
	rec = api.do("GET", "/api/v1/jobs?status=bogus", api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "unknown status")
}

func TestJobCreateUnknownCustomer(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("POST", "/api/v1/jobs", api.staff, map[string]any{
		"customer_id": 4242, "title": "Ghost job",
	})
	body := api.mustError(rec, http.StatusBadRequest, "validation_failed", "customer_id must exist")
	if body.Error.Fields["customer_id"] == "" {
		t.Fatalf("fields: %v", body.Error.Fields)
	}
}
