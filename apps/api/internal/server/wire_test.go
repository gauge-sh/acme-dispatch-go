package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListEnvelopeKeyOrder(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/customers", api.staff, nil)
	api.mustStatus(rec, http.StatusOK)
	body := rec.Body.String()
	// The envelope is items, page, per_page, total — in that order.
	positions := []int{
		strings.Index(body, `"items"`),
		strings.Index(body, `"page"`),
		strings.Index(body, `"per_page"`),
		strings.Index(body, `"total"`),
	}
	for i, pos := range positions {
		if pos == -1 {
			t.Fatalf("envelope key %d missing: %s", i, body)
		}
		if i > 0 && positions[i-1] > pos {
			t.Fatalf("envelope keys out of order: %s", body)
		}
	}
}

func TestContentTypeWithCharset(t *testing.T) {
	api := newTestAPI(t)
	raw, _ := json.Marshal(map[string]any{"name": "Harbor", "service_address": "1 Pier Rd"})
	req := httptest.NewRequest("POST", "/api/v1/customers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+api.staff)
	rec := httptest.NewRecorder()
	api.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("charset parameter rejected: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/customers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer "+api.staff)
	rec = httptest.NewRecorder()
	api.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("text/plain accepted: %d", rec.Code)
	}
}

func TestAppointmentPatchIsPartial(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana", 8000)
	job := api.createJob(customer["id"], "Fix furnace", "")
	appt := api.createAppointment(job["id"], tech["id"], "2025-03-12T09:00:00Z", "2025-03-12T11:00:00Z")

	// Only the notes change; the window and technician stay put.
	rec := api.do("PATCH", jsonPath("/api/v1/appointments/%v", appt["id"]), api.staff, map[string]any{
		"completion_notes": "replaced igniter",
	})
	api.mustStatus(rec, http.StatusOK)
	patched := decodeBody[map[string]any](t, rec)
	if patched["scheduled_start"] != "2025-03-12T09:00:00Z" || patched["completion_notes"] != "replaced igniter" {
		t.Fatalf("partial patch: %v", patched)
	}

	// Explicit null clears the nullable field.
	rec = api.do("PATCH", jsonPath("/api/v1/appointments/%v", appt["id"]), api.staff, map[string]any{
		"completion_notes": nil,
	})
	patched = decodeBody[map[string]any](t, rec)
	if patched["completion_notes"] != nil {
		t.Fatalf("null clear: %v", patched)
	}

	// Null on a required field is refused.
	rec = api.do("PATCH", jsonPath("/api/v1/appointments/%v", appt["id"]), api.staff, map[string]any{
		"technician_id": nil,
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "technician_id must not be null")
}

func TestBadQueryParams(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/jobs?customer_id=abc", api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "customer_id must be an integer")

	rec = api.do("GET", "/api/v1/technicians?is_active=yes", api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "is_active must be true or false")

	rec = api.do("GET", "/api/v1/stock-levels?part_id=1.5", api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "part_id must be an integer")
}

func TestUserPasswordChangeOverAPI(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("POST", "/api/v1/users", api.admin, map[string]any{
		"email": "rotate@test.example", "name": "Rotate Me", "password": "first-password",
	})
	api.mustStatus(rec, http.StatusCreated)
	created := decodeBody[map[string]any](t, rec)

	if u, _ := api.store.Authenticate("rotate@test.example", "first-password"); u == nil {
		t.Fatal("initial password does not work")
	}

	rec = api.do("PATCH", jsonPath("/api/v1/users/%v", created["id"]), api.admin, map[string]any{
		"password": "second-password",
	})
	api.mustStatus(rec, http.StatusOK)
	if u, _ := api.store.Authenticate("rotate@test.example", "second-password"); u == nil {
		t.Fatal("new password does not work")
	}
	if u, _ := api.store.Authenticate("rotate@test.example", "first-password"); u != nil {
		t.Fatal("old password still works")
	}
}

func TestTechnicianCombinedFilters(t *testing.T) {
	api := newTestAPI(t)
	api.createTechnician("Dana", 8000) // hvac, from the harness helper
	plumber := api.createTechnician("Iris", 7200)
	rec := api.do("PATCH", jsonPath("/api/v1/technicians/%v", plumber["id"]), api.staff, map[string]any{
		"skill_tags": []string{"plumbing"}, "is_active": false,
	})
	api.mustStatus(rec, http.StatusOK)

	rec = api.do("GET", "/api/v1/technicians?skill=plumbing&is_active=false", api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Total != 1 || list.Items[0]["name"] != "Iris" {
		t.Fatalf("combined filter: %+v", list)
	}
	rec = api.do("GET", "/api/v1/technicians?skill=plumbing&is_active=true", api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 0 {
		t.Fatalf("no active plumbers expected: %+v", list)
	}
}

func TestJobsPageBeyondEnd(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	api.createJob(customer["id"], "Only job", "")

	rec := api.do("GET", "/api/v1/jobs?page=40&per_page=100", api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if len(list.Items) != 0 || list.Total != 1 || list.Page != 40 {
		t.Fatalf("past the end: %+v", list)
	}
}
