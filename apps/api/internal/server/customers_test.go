package server

import (
	"net/http"
	"testing"
)

func TestCustomerLifecycle(t *testing.T) {
	api := newTestAPI(t)

	rec := api.do("POST", "/api/v1/customers", api.staff, map[string]any{
		"name":            "Harbor Coffee",
		"service_address": "12 Pier Rd",
		"contact_email":   "ops@harbor.test",
	})
	api.mustStatus(rec, http.StatusCreated)
	created := decodeBody[map[string]any](t, rec)
	if created["id"] == nil || created["created_at"] == nil {
		t.Fatalf("missing envelope fields: %v", created)
	}
	if created["phone"] != nil || created["notes"] != nil {
		t.Fatalf("nullable fields should be null: %v", created)
	}

	rec = api.do("GET", jsonPath("/api/v1/customers/%v", created["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusOK)

	// Partial update: absent keys stay put, explicit null clears.
	rec = api.do("PATCH", jsonPath("/api/v1/customers/%v", created["id"]), api.staff, map[string]any{
		"phone":         "555-0100",
		"contact_email": nil,
	})
	api.mustStatus(rec, http.StatusOK)
	patched := decodeBody[map[string]any](t, rec)
	if patched["phone"] != "555-0100" || patched["contact_email"] != nil {
		t.Fatalf("patch semantics: %v", patched)
	}
	if patched["name"] != "Harbor Coffee" {
		t.Fatalf("absent key overwritten: %v", patched)
	}

	// Null on a required field is refused.
	rec = api.do("PATCH", jsonPath("/api/v1/customers/%v", created["id"]), api.staff, map[string]any{"name": nil})
	body := api.mustError(rec, http.StatusBadRequest, "validation_failed", "name must not be null")
	if body.Error.Fields["name"] == "" {
		t.Fatalf("fields missing: %v", body.Error.Fields)
	}

	rec = api.do("DELETE", jsonPath("/api/v1/customers/%v", created["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Fatal("204 must have an empty body")
	}

	rec = api.do("GET", jsonPath("/api/v1/customers/%v", created["id"]), api.staff, nil)
	api.mustError(rec, http.StatusNotFound, "not_found", "customer not found")
}

func TestCustomerValidationShape(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("POST", "/api/v1/customers", api.staff, map[string]any{
		"name": "", "service_address": "", "contact_email": "bogus",
	})
	body := api.mustError(rec, http.StatusBadRequest, "validation_failed", "")
	for _, field := range []string{"name", "service_address", "contact_email"} {
		if body.Error.Fields[field] == "" {
			t.Errorf("missing field error for %s: %v", field, body.Error.Fields)
		}
	}
}

func TestListEnvelopeAndPagination(t *testing.T) {
	api := newTestAPI(t)
	for _, name := range []string{"Alpha Co", "Beta LLC", "Gamma Inc", "Delta Corp", "Harbor East"} {
		api.createCustomer(name)
	}

	rec := api.do("GET", "/api/v1/customers?page=2&per_page=2", api.staff, nil)
	api.mustStatus(rec, http.StatusOK)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Page != 2 || list.PerPage != 2 || list.Total != 5 || len(list.Items) != 2 {
		t.Fatalf("envelope: %+v", list)
	}
	if list.Items[0]["name"] != "Gamma Inc" {
		t.Fatalf("id ordering: %v", list.Items[0])
	}

	// Beyond the end: empty items, true total.
	rec = api.do("GET", "/api/v1/customers?page=9", api.staff, nil)
	list = decodeBody[listBody[map[string]any]](t, rec)
	if len(list.Items) != 0 || list.Total != 5 {
		t.Fatalf("past the end: %+v", list)
	}

	// per_page above 100 clamps quietly.
	rec = api.do("GET", "/api/v1/customers?per_page=500", api.staff, nil)
	list = decodeBody[listBody[map[string]any]](t, rec)
	if list.PerPage != 100 {
		t.Fatalf("clamp: %d", list.PerPage)
	}

	for _, bad := range []string{"page=0", "page=-1", "page=abc", "per_page=0", "per_page=x"} {
		rec := api.do("GET", "/api/v1/customers?"+bad, api.staff, nil)
		api.mustError(rec, http.StatusBadRequest, "validation_failed", "page and per_page must be positive integers")
	}

	// Substring filter is case-insensitive over name and address.
	rec = api.do("GET", "/api/v1/customers?q=harbor", api.staff, nil)
	list = decodeBody[listBody[map[string]any]](t, rec)
	if list.Total != 1 {
		t.Fatalf("q filter: %+v", list)
	}
}

func TestDeleteCustomerWithJobsFails(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Busy Corp")
	api.createJob(customer["id"], "Fix furnace", "")

	rec := api.do("DELETE", jsonPath("/api/v1/customers/%v", customer["id"]), api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "customer has jobs and cannot be deleted")
}
