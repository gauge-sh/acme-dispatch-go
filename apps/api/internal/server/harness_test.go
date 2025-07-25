package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type testAPI struct {
	t       *testing.T
	handler http.Handler
	store   *store.Store
	admin   string // bearer token
	staff   string // dispatcher bearer token
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	admin := core.User{Email: "admin@test.example", Name: "Admin", Role: core.RoleAdmin, IsActive: true}
	if err := s.CreateUser(&admin, "admin-password"); err != nil {
		t.Fatal(err)
	}
	staff := core.User{Email: "staff@test.example", Name: "Staff", Role: core.RoleDispatcher, IsActive: true}
	if err := s.CreateUser(&staff, "staff-password"); err != nil {
		t.Fatal(err)
	}
	adminToken, _, err := s.IssueToken(admin.ID, "tests")
	if err != nil {
		t.Fatal(err)
	}
	staffToken, _, err := s.IssueToken(staff.ID, "tests")
	if err != nil {
		t.Fatal(err)
	}
	return &testAPI{t: t, handler: New(s), store: s, admin: adminToken, staff: staffToken}
}

func (a *testAPI) do(method, path, token string, body any) *httptest.ResponseRecorder {
	a.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func (a *testAPI) mustStatus(rec *httptest.ResponseRecorder, want int) {
	a.t.Helper()
	if rec.Code != want {
		a.t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body.String())
	}
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

type apiError struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	} `json:"error"`
}

func (a *testAPI) mustError(rec *httptest.ResponseRecorder, status int, code, message string) apiError {
	a.t.Helper()
	a.mustStatus(rec, status)
	body := decodeBody[apiError](a.t, rec)
	if body.Error.Code != code {
		a.t.Fatalf("code %q, want %q (%s)", body.Error.Code, code, rec.Body.String())
	}
	if message != "" && body.Error.Message != message {
		a.t.Fatalf("message %q, want %q", body.Error.Message, message)
	}
	return body
}

type listBody[T any] struct {
	Items   []T `json:"items"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// createCustomer and friends drive the API itself, so fixtures exercise the
// same paths the assertions do.
func (a *testAPI) createCustomer(name string) map[string]any {
	rec := a.do("POST", "/api/v1/customers", a.staff, map[string]any{
		"name": name, "service_address": "1 Main St",
	})
	a.mustStatus(rec, http.StatusCreated)
	return decodeBody[map[string]any](a.t, rec)
}

func (a *testAPI) createTechnician(name string, rate int64) map[string]any {
	rec := a.do("POST", "/api/v1/technicians", a.staff, map[string]any{
		"name": name, "hourly_cost_cents": rate, "skill_tags": []string{"hvac"},
	})
	a.mustStatus(rec, http.StatusCreated)
	return decodeBody[map[string]any](a.t, rec)
}

func (a *testAPI) createPart(sku string, price int64, onHand int64) map[string]any {
	rec := a.do("POST", "/api/v1/parts", a.staff, map[string]any{
		"sku": sku, "name": "Part " + sku, "unit_cost_cents": 500, "unit_price_cents": price,
	})
	a.mustStatus(rec, http.StatusCreated)
	part := decodeBody[map[string]any](a.t, rec)
	if onHand > 0 {
		rec := a.do("POST", jsonPath("/api/v1/stock-levels/%v/adjust", part["id"]), a.staff, map[string]any{"delta": onHand})
		a.mustStatus(rec, http.StatusOK)
	}
	return part
}

func (a *testAPI) createJob(customerID any, title string, priority string) map[string]any {
	body := map[string]any{"customer_id": customerID, "title": title}
	if priority != "" {
		body["priority"] = priority
	}
	rec := a.do("POST", "/api/v1/jobs", a.staff, body)
	a.mustStatus(rec, http.StatusCreated)
	return decodeBody[map[string]any](a.t, rec)
}

func (a *testAPI) createAppointment(jobID, techID any, start, end string) map[string]any {
	rec := a.do("POST", "/api/v1/appointments", a.staff, map[string]any{
		"job_id": jobID, "technician_id": techID,
		"scheduled_start": start, "scheduled_end": end,
	})
	a.mustStatus(rec, http.StatusCreated)
	return decodeBody[map[string]any](a.t, rec)
}

func (a *testAPI) transition(jobID any, status string) *httptest.ResponseRecorder {
	return a.do("POST", jsonPath("/api/v1/jobs/%v/transition", jobID), a.staff, map[string]any{"status": status})
}

// jsonPath formats ids that came back through JSON, where numbers land as
// float64.
func jsonPath(format string, args ...any) string {
	for i, arg := range args {
		if f, ok := arg.(float64); ok {
			args[i] = int64(f)
		}
	}
	return fmt.Sprintf(format, args...)
}
