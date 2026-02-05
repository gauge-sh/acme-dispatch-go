package server

import (
	"net/http"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func TestAuthRequired(t *testing.T) {
	api := newTestAPI(t)

	rec := api.do("GET", "/api/v1/customers", "", nil)
	api.mustError(rec, http.StatusUnauthorized, "unauthorized", "invalid or missing token")

	rec = api.do("GET", "/api/v1/customers", "not-a-real-token", nil)
	api.mustError(rec, http.StatusUnauthorized, "unauthorized", "invalid or missing token")

	req := api.do("GET", "/api/v1/customers", api.staff, nil)
	api.mustStatus(req, http.StatusOK)
}

func TestInactiveUserTokenStopsWorking(t *testing.T) {
	api := newTestAPI(t)

	ghost := core.User{Email: "ghost@test.example", Name: "Ghost", Role: core.RoleDispatcher, IsActive: true}
	if err := api.store.CreateUser(&ghost, "ghost-password"); err != nil {
		t.Fatal(err)
	}
	token, _, err := api.store.IssueToken(ghost.ID, "soon dead")
	if err != nil {
		t.Fatal(err)
	}
	api.mustStatus(api.do("GET", "/api/v1/jobs", token, nil), http.StatusOK)

	rec := api.do("PATCH", jsonPath("/api/v1/users/%v", ghost.ID), api.admin, map[string]any{"is_active": false})
	api.mustStatus(rec, http.StatusOK)

	rec = api.do("GET", "/api/v1/jobs", token, nil)
	api.mustError(rec, http.StatusUnauthorized, "unauthorized", "")
}

func TestAdminOnlyRoutes(t *testing.T) {
	api := newTestAPI(t)

	// Everything under /users is admin-only.
	rec := api.do("GET", "/api/v1/users", api.staff, nil)
	api.mustError(rec, http.StatusForbidden, "forbidden", "admin access required")
	api.mustStatus(api.do("GET", "/api/v1/users", api.admin, nil), http.StatusOK)

	// So are technician and part deletes; other verbs stay open.
	tech := api.createTechnician("Dana", 5000)
	rec = api.do("DELETE", jsonPath("/api/v1/technicians/%v", tech["id"]), api.staff, nil)
	api.mustError(rec, http.StatusForbidden, "forbidden", "")
	api.mustStatus(api.do("DELETE", jsonPath("/api/v1/technicians/%v", tech["id"]), api.admin, nil), http.StatusNoContent)

	part := api.createPart("FLT-1", 899, 0)
	rec = api.do("DELETE", jsonPath("/api/v1/parts/%v", part["id"]), api.staff, nil)
	api.mustError(rec, http.StatusForbidden, "forbidden", "")
	api.mustStatus(api.do("DELETE", jsonPath("/api/v1/parts/%v", part["id"]), api.admin, nil), http.StatusNoContent)
}

func TestUnknownRouteIsJSON(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/nope", api.staff, nil)
	api.mustError(rec, http.StatusNotFound, "not_found", "not found")
}

func TestContentTypeAndMalformedBody(t *testing.T) {
	api := newTestAPI(t)

	req := api.do("POST", "/api/v1/customers", api.staff, nil) // no body at all
	api.mustError(req, http.StatusBadRequest, "validation_failed", "invalid request body")

	raw := api.do("POST", "/api/v1/customers", api.staff, "just a string")
	api.mustError(raw, http.StatusBadRequest, "validation_failed", "invalid request body")
}

func TestBearerPrefixIsExact(t *testing.T) {
	api := newTestAPI(t)
	for _, header := range []string{
		"bearer " + api.staff, // lower-case scheme
		"Token " + api.staff,
		api.staff, // bare token
		"Bearer",
		"Bearer ",
	} {
		rec := api.doWithAuthHeader("GET", "/api/v1/jobs", header)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q got %d", header, rec.Code)
		}
	}
}
