package server

import (
	"net/http"
	"strings"
	"testing"
)

// caseFourJob builds a job shaped like conformance case 4: one 1.5h weekday
// appointment at 8000¢/h, 3 parts at 1299¢, urgent priority.
func caseFourJob(t *testing.T, api *testAPI, sku string) map[string]any {
	t.Helper()
	customer := api.createCustomer("Harbor Coffee")
	tech := api.createTechnician("Dana Reyes", 8000)
	part := api.createPart(sku, 1299, 10)
	job := api.createJob(customer["id"], "Furnace not igniting", "urgent")

	api.createAppointment(job["id"], tech["id"], "2025-03-12T13:00:00Z", "2025-03-12T14:30:00Z")
	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 3,
	})
	api.mustStatus(rec, http.StatusCreated)

	for _, status := range []string{"scheduled", "in_progress", "completed"} {
		api.mustStatus(api.transition(job["id"], status), http.StatusOK)
	}
	return job
}

func TestInvoiceCaseFourOverAPI(t *testing.T) {
	api := newTestAPI(t)
	job := caseFourJob(t, api, "IGN-4420")

	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", job["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusCreated)
	rawInvoice := rec.Body.String()
	invoice := decodeBody[map[string]any](t, rec)

	if invoice["labor_cents"] != float64(19200) || invoice["parts_cents"] != float64(3897) {
		t.Fatalf("subtotals: %v", invoice)
	}
	if invoice["total_cents"] != float64(25407) {
		t.Fatalf("case 4 total: %v", invoice["total_cents"])
	}
	lines := invoice["line_items"].([]any)
	if len(lines) != 2 {
		t.Fatalf("line items: %v", lines)
	}
	labor := lines[0].(map[string]any)
	if labor["kind"] != "labor" || labor["description"] != "Dana Reyes — 1.5h on 2025-03-12" {
		t.Fatalf("labor line: %v", labor)
	}
	if labor["quantity"] != float64(1.5) || labor["weekend"] != false {
		t.Fatalf("labor line: %v", labor)
	}
	partLine := lines[1].(map[string]any)
	if partLine["kind"] != "part" || partLine["description"] != "IGN-4420 — Part IGN-4420" {
		t.Fatalf("part line: %v", partLine)
	}
	if partLine["quantity"] != float64(3) || partLine["amount_cents"] != float64(3897) {
		t.Fatalf("part line: %v", partLine)
	}

	// The job moved with the invoice.
	rec = api.do("GET", jsonPath("/api/v1/jobs/%v", job["id"]), api.staff, nil)
	detail := decodeBody[map[string]any](t, rec)
	if detail["status"] != "invoiced" || detail["invoice_record_id"] == nil {
		t.Fatalf("job after invoice: %v", detail)
	}

	// Bare-number encoding: 1.5 and 3, never "1.5" or 3.0.
	if !strings.Contains(rawInvoice, `"quantity":1.5`) || !strings.Contains(rawInvoice, `"quantity":3`) {
		t.Fatalf("quantity encoding: %s", rawInvoice)
	}
	if strings.Contains(rawInvoice, `"quantity":"`) || strings.Contains(rawInvoice, `"quantity":3.0`) {
		t.Fatalf("quantity encoding: %s", rawInvoice)
	}
}

func TestInvoicePrecedenceOverAPI(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	job := api.createJob(customer["id"], "Fix furnace", "")

	// Wrong status first: 409 invalid_transition.
	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", job["id"]), api.staff, nil)
	api.mustError(rec, http.StatusConflict, "invalid_transition", "only a completed job can be invoiced")

	done := caseFourJob(t, api, "IGN-B100")
	api.mustStatus(api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", done["id"]), api.staff, nil), http.StatusCreated)

	// Already invoiced: the duplicate check outranks the status check, 400.
	rec = api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", done["id"]), api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "job is already invoiced")

	// And the invoiced job can no longer be deleted or changed.
	rec = api.do("DELETE", jsonPath("/api/v1/jobs/%v", done["id"]), api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "an invoiced job cannot be deleted")

	detail := decodeBody[map[string]any](t, api.do("GET", jsonPath("/api/v1/jobs/%v", done["id"]), api.staff, nil))
	jp := detail["job_parts"].([]any)[0].(map[string]any)
	rec = api.do("DELETE", jsonPath("/api/v1/job-parts/%v", jp["id"]), api.staff, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "an invoiced job cannot be changed")
}

func TestInvoiceListAndGet(t *testing.T) {
	api := newTestAPI(t)
	first := caseFourJob(t, api, "IGN-C100")
	api.mustStatus(api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", first["id"]), api.staff, nil), http.StatusCreated)
	second := caseFourJob(t, api, "IGN-D100")
	api.mustStatus(api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", second["id"]), api.staff, nil), http.StatusCreated)

	rec := api.do("GET", "/api/v1/invoice-records", api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Total != 2 {
		t.Fatalf("total: %+v", list)
	}
	if list.Items[0]["id"].(float64) >= list.Items[1]["id"].(float64) {
		t.Fatalf("invoice ordering: %v", list.Items)
	}
	if _, ok := list.Items[0]["line_items"].([]any); !ok {
		t.Fatalf("list items include line_items: %v", list.Items[0])
	}

	rec = api.do("GET", jsonPath("/api/v1/invoice-records?job_id=%v", first["id"]), api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 1 {
		t.Fatalf("job filter: %+v", list)
	}
	rec = api.do("GET", jsonPath("/api/v1/invoice-records?customer_id=%v", second["customer_id"]), api.staff, nil)
	if list := decodeBody[listBody[map[string]any]](t, rec); list.Total != 1 {
		t.Fatalf("customer filter: %+v", list)
	}

	id := decodeBody[listBody[map[string]any]](t, api.do("GET", "/api/v1/invoice-records", api.staff, nil)).Items[0]["id"]
	rec = api.do("GET", jsonPath("/api/v1/invoice-records/%v", id), api.staff, nil)
	api.mustStatus(rec, http.StatusOK)

	rec = api.do("GET", "/api/v1/invoice-records/999", api.staff, nil)
	api.mustError(rec, http.StatusNotFound, "not_found", "invoice record not found")
}

func TestWeekendLineOverAPI(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Weekend Work Co")
	tech := api.createTechnician("Yuki", 5000)
	job := api.createJob(customer["id"], "Saturday call-out", "")

	// 2025-03-15 is a Saturday.
	api.createAppointment(job["id"], tech["id"], "2025-03-15T09:00:00Z", "2025-03-15T11:00:00Z")
	for _, status := range []string{"scheduled", "in_progress", "completed"} {
		api.mustStatus(api.transition(job["id"], status), http.StatusOK)
	}

	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/invoice", job["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusCreated)
	invoice := decodeBody[map[string]any](t, rec)
	// 2h × 5000 × 1.6 = 16000, ×1.15 = 18400: fixture case 2.
	if invoice["total_cents"] != float64(18400) {
		t.Fatalf("saturday total: %v", invoice["total_cents"])
	}
	line := invoice["line_items"].([]any)[0].(map[string]any)
	if line["weekend"] != true || line["amount_cents"] != float64(18400) {
		t.Fatalf("weekend line: %v", line)
	}
}
