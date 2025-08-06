package server

import (
	"net/http"
	"testing"
)

func TestUserAdminCRUD(t *testing.T) {
	api := newTestAPI(t)

	rec := api.do("POST", "/api/v1/users", api.admin, map[string]any{
		"email": "New.Person@Test.example", "name": "New Person", "password": "a-long-password",
	})
	api.mustStatus(rec, http.StatusCreated)
	created := decodeBody[map[string]any](t, rec)
	if created["email"] != "new.person@test.example" {
		t.Fatalf("email not normalized: %v", created)
	}
	if created["role"] != "dispatcher" || created["is_active"] != true {
		t.Fatalf("defaults: %v", created)
	}
	if _, leaked := created["password"]; leaked {
		t.Fatal("password serialized")
	}
	if _, leaked := created["password_hash"]; leaked {
		t.Fatal("password hash serialized")
	}

	rec = api.do("POST", "/api/v1/users", api.admin, map[string]any{
		"email": "NEW.PERSON@test.example", "name": "Duplicate",
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "email is already in use")

	rec = api.do("PATCH", jsonPath("/api/v1/users/%v", created["id"]), api.admin, map[string]any{"role": "admin"})
	api.mustStatus(rec, http.StatusOK)
	if patched := decodeBody[map[string]any](t, rec); patched["role"] != "admin" {
		t.Fatalf("promotion lost: %v", patched)
	}

	rec = api.do("GET", "/api/v1/users?role=admin", api.admin, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 2 {
		t.Fatalf("role filter: %+v", list)
	}

	rec = api.do("DELETE", jsonPath("/api/v1/users/%v", created["id"]), api.admin, nil)
	api.mustStatus(rec, http.StatusNoContent)
}

func TestUserSelfServiceRulesOverAPI(t *testing.T) {
	api := newTestAPI(t)
	me := decodeBody[listBody[map[string]any]](t, api.do("GET", "/api/v1/users?role=admin", api.admin, nil)).Items[0]

	rec := api.do("PATCH", jsonPath("/api/v1/users/%v", me["id"]), api.admin, map[string]any{"role": "dispatcher"})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "you cannot change your own role")

	rec = api.do("PATCH", jsonPath("/api/v1/users/%v", me["id"]), api.admin, map[string]any{"is_active": false})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "you cannot deactivate yourself")

	// The only active admin cannot be deleted, even by themselves.
	rec = api.do("DELETE", jsonPath("/api/v1/users/%v", me["id"]), api.admin, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "cannot remove the last active admin")

	// A name edit on yourself is fine.
	rec = api.do("PATCH", jsonPath("/api/v1/users/%v", me["id"]), api.admin, map[string]any{"name": "Renamed Admin"})
	api.mustStatus(rec, http.StatusOK)
}

func TestUnknownUserRoutes(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/users/999", api.admin, nil)
	api.mustError(rec, http.StatusNotFound, "not_found", "user not found")
	rec = api.do("PATCH", "/api/v1/users/999", api.admin, map[string]any{"name": "x"})
	api.mustError(rec, http.StatusNotFound, "not_found", "user not found")
	rec = api.do("DELETE", "/api/v1/users/999", api.admin, nil)
	api.mustError(rec, http.StatusNotFound, "not_found", "user not found")
}
