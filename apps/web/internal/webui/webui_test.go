package webui

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type testApp struct {
	t      *testing.T
	server *httptest.Server
	client *http.Client
	store  *store.Store
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	admin := core.User{Email: "admin@test.example", Name: "Admin", Role: core.RoleAdmin, IsActive: true}
	if err := s.CreateUser(&admin, "admin-password"); err != nil {
		t.Fatal(err)
	}
	staff := core.User{Email: "staff@test.example", Name: "Staff", Role: core.RoleDispatcher, IsActive: true}
	if err := s.CreateUser(&staff, "staff-password"); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(New(s, []byte("test-session-secret")))
	jar, _ := cookiejar.New(nil)
	t.Cleanup(func() {
		server.Close()
		s.Close()
	})
	return &testApp{t: t, server: server, client: &http.Client{Jar: jar}, store: s}
}

func (app *testApp) login(email, password string) {
	app.t.Helper()
	resp := app.postForm("/login", url.Values{"email": {email}, "password": {password}})
	defer resp.Body.Close()
	if resp.Request.URL.Path != "/" {
		app.t.Fatalf("login landed on %s", resp.Request.URL.Path)
	}
}

func (app *testApp) get(path string) (*http.Response, string) {
	app.t.Helper()
	resp, err := app.client.Get(app.server.URL + path)
	if err != nil {
		app.t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

func (app *testApp) postForm(path string, form url.Values) *http.Response {
	app.t.Helper()
	resp, err := app.client.PostForm(app.server.URL+path, form)
	if err != nil {
		app.t.Fatal(err)
	}
	return resp
}

func (app *testApp) postAndRead(path string, form url.Values) (*http.Response, string) {
	app.t.Helper()
	resp := app.postForm(path, form)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

func mustContain(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("page missing %q:\n%s", want, body[:min(len(body), 2000)])
	}
}

func TestLoginFlow(t *testing.T) {
	app := newTestApp(t)

	// Signed out: everything bounces to the login form.
	resp, body := app.get("/jobs")
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("expected login redirect, got %s", resp.Request.URL.Path)
	}
	mustContain(t, body, "Sign in")

	resp, body = app.postAndRead("/login", url.Values{
		"email": {"admin@test.example"}, "password": {"wrong"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad login status %d", resp.StatusCode)
	}
	mustContain(t, body, "invalid email or password")

	app.login("admin@test.example", "admin-password")
	_, body = app.get("/")
	mustContain(t, body, "Dashboard")

	// Sign out and confirm the door is shut again.
	resp = app.postForm("/logout", url.Values{})
	resp.Body.Close()
	resp, _ = app.get("/")
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("logout did not clear the session: %s", resp.Request.URL.Path)
	}
}

func TestInactiveUserCannotLogIn(t *testing.T) {
	app := newTestApp(t)
	ghost := core.User{Email: "ghost@test.example", Name: "Ghost", Role: core.RoleDispatcher, IsActive: false}
	if err := app.store.CreateUser(&ghost, "ghost-password"); err != nil {
		t.Fatal(err)
	}
	resp, body := app.postAndRead("/login", url.Values{
		"email": {"ghost@test.example"}, "password": {"ghost-password"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", resp.StatusCode)
	}
	mustContain(t, body, "invalid email or password")
}

func TestCustomerFormFlow(t *testing.T) {
	app := newTestApp(t)
	app.login("staff@test.example", "staff-password")

	_, body := app.get("/customers")
	mustContain(t, body, "No customers yet")

	// A bad submit re-renders the form with what was typed.
	resp, body := app.postAndRead("/customers", url.Values{
		"name": {""}, "service_address": {"12 Pier Rd"}, "contact_email": {"nope"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", resp.StatusCode)
	}
	mustContain(t, body, "12 Pier Rd")
	mustContain(t, body, "must be an email address")

	resp, body = app.postAndRead("/customers", url.Values{
		"name": {"Harbor Coffee"}, "service_address": {"12 Pier Rd"}, "notes": {"gate code 4412"},
	})
	mustContain(t, body, "Customer created.")
	mustContain(t, body, "Harbor Coffee")
	mustContain(t, body, "gate code 4412")

	_, body = app.get("/customers?q=harbor")
	mustContain(t, body, "Harbor Coffee")
	_, body = app.get("/customers?q=zzz")
	mustContain(t, body, "No customers match")
}

func TestJobLifecycleInBrowser(t *testing.T) {
	app := newTestApp(t)
	app.login("staff@test.example", "staff-password")

	customer := core.Customer{Name: "Harbor Coffee", ServiceAddress: "12 Pier Rd"}
	if err := app.store.CreateCustomer(&customer); err != nil {
		t.Fatal(err)
	}
	tech := core.Technician{Name: "Dana Reyes", HourlyCostCents: 8000, IsActive: true}
	if err := app.store.CreateTechnician(&tech); err != nil {
		t.Fatal(err)
	}
	part := core.Part{SKU: "IGN-4420", Name: "Hot surface igniter", UnitCostCents: 800, UnitPriceCents: 1299}
	if err := app.store.CreatePart(&part); err != nil {
		t.Fatal(err)
	}
	if _, err := app.store.AdjustStockLevel(part.ID, 10, nil); err != nil {
		t.Fatal(err)
	}

	resp, body := app.postAndRead("/jobs", url.Values{
		"customer_id": {strconv.FormatInt(customer.ID, 10)}, "title": {"Furnace not igniting"}, "priority": {"urgent"},
	})
	if resp.Request.URL.Path == "/jobs" {
		t.Fatalf("job create failed: %s", body)
	}
	jobPath := resp.Request.URL.Path
	mustContain(t, body, "Furnace not igniting")
	mustContain(t, body, "urgent")

	// Scheduling needs an appointment first; the error surfaces as a flash.
	_, body = app.postAndRead(jobPath+"/transition", url.Values{"status": {"scheduled"}})
	mustContain(t, body, "a job needs an appointment before it can be scheduled")

	resp, body = app.postAndRead(jobPath+"/appointments", url.Values{
		"technician_id":   {strconv.FormatInt(tech.ID, 10)},
		"scheduled_start": {"2025-03-12T13:00"},
		"scheduled_end":   {"2025-03-12T14:30"},
	})
	mustContain(t, body, "Appointment scheduled.")

	// A second appointment in the same window shows the conflict on the form.
	resp, body = app.postAndRead(jobPath+"/appointments", url.Values{
		"technician_id":   {strconv.FormatInt(tech.ID, 10)},
		"scheduled_start": {"2025-03-12T13:30"},
		"scheduled_end":   {"2025-03-12T15:00"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("conflict status %d", resp.StatusCode)
	}
	mustContain(t, body, "technician is already booked for that time")

	for _, status := range []string{"scheduled", "in_progress", "completed"} {
		_, body = app.postAndRead(jobPath+"/transition", url.Values{"status": {status}})
	}
	mustContain(t, body, "Job moved to completed.")

	_, body = app.postAndRead(jobPath+"/parts", url.Values{
		"part_id": {strconv.FormatInt(part.ID, 10)}, "quantity": {"3"},
	})
	mustContain(t, body, "Part recorded and taken from stock.")

	resp, body = app.postAndRead(jobPath+"/invoice", url.Values{})
	if !strings.Contains(resp.Request.URL.Path, "/invoices/") {
		t.Fatalf("invoice landed on %s", resp.Request.URL.Path)
	}
	// Case 4 numbers, straight from the seeded shapes.
	mustContain(t, body, "$254.07")
	mustContain(t, body, "Dana Reyes — 1.5h on 2025-03-12")

	// The job page now links the invoice and offers no transitions.
	_, body = app.get(jobPath)
	mustContain(t, body, "View invoice")
}

func TestAdminGatingInBrowser(t *testing.T) {
	app := newTestApp(t)
	app.login("staff@test.example", "staff-password")

	resp, _ := app.get("/users")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("dispatcher reached /users: %d", resp.StatusCode)
	}

	admin := newTestApp(t)
	admin.login("admin@test.example", "admin-password")
	resp, body := admin.get("/users")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin blocked from /users: %d", resp.StatusCode)
	}
	mustContain(t, body, "New user")
}

func TestTokenShownExactlyOnce(t *testing.T) {
	app := newTestApp(t)
	app.login("admin@test.example", "admin-password")

	_, listBody := app.get("/users")
	userPath := "/users/2" // the dispatcher seeded in newTestApp
	mustContain(t, listBody, "Staff")

	resp, body := app.postAndRead(userPath+"/tokens", url.Values{"name": {"reporting script"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("issue status %d", resp.StatusCode)
	}
	mustContain(t, body, "shown only this once")
	start := strings.Index(body, "<code>")
	end := strings.Index(body, "</code>")
	if start == -1 || end == -1 {
		t.Fatal("no token rendered")
	}
	plaintext := body[start+6 : end]
	if len(plaintext) != 64 {
		t.Fatalf("token length %d", len(plaintext))
	}

	// Reloading the page must not show it again.
	_, body = app.get(userPath)
	if strings.Contains(body, plaintext) {
		t.Fatal("token shown twice")
	}
	mustContain(t, body, "reporting script")

	// And the token actually authenticates against the store.
	user, err := app.store.UserForToken(plaintext)
	if err != nil || user == nil {
		t.Fatalf("issued token does not resolve: %v %v", user, err)
	}
}

func TestSeededPagesRender(t *testing.T) {
	app := newTestApp(t)
	if err := app.store.Seed(); err != nil {
		t.Fatal(err)
	}
	app.login("admin@acme-dispatch.example", "dispatch-admin")

	for _, path := range []string{
		"/", "/jobs", "/jobs?status=invoiced", "/jobs/1", "/customers", "/customers/1",
		"/technicians", "/technicians/1", "/parts", "/parts/1", "/invoices", "/users",
	} {
		resp, body := app.get(path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		if !strings.Contains(body, "Acme Dispatch") {
			t.Errorf("%s: layout missing", path)
		}
	}

	// Invoice 1 belongs to a seeded invoiced job; the page shows real money.
	_, body := app.get("/invoices/1")
	mustContain(t, body, "Invoice record #1")
	mustContain(t, body, "$")
}

func TestCrossSitePostsRefused(t *testing.T) {
	app := newTestApp(t)
	app.login("staff@test.example", "staff-password")

	req, _ := http.NewRequest("POST", app.server.URL+"/customers",
		strings.NewReader("name=Evil&service_address=1+Bad+St"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := app.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site post got %d", resp.StatusCode)
	}

	// Same-origin posts are untouched.
	req, _ = http.NewRequest("POST", app.server.URL+"/customers",
		strings.NewReader("name=Fine&service_address=1+Good+St"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err = app.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin post got %d", resp.StatusCode)
	}
}
