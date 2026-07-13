package server

import (
	"net/http"
	"testing"
)

func TestStockAdjustEndpoint(t *testing.T) {
	api := newTestAPI(t)
	part := api.createPart("FLT-1", 899, 0)

	rec := api.do("POST", jsonPath("/api/v1/stock-levels/%v/adjust", part["id"]), api.staff, map[string]any{"delta": 10})
	api.mustStatus(rec, http.StatusOK)
	level := decodeBody[map[string]any](t, rec)
	if level["quantity_on_hand"] != float64(10) {
		t.Fatalf("adjust result: %v", level)
	}

	rec = api.do("POST", jsonPath("/api/v1/stock-levels/%v/adjust", part["id"]), api.staff, map[string]any{"delta": -11})
	api.mustError(rec, http.StatusConflict, "insufficient_stock", "not enough stock on hand")

	rec = api.do("POST", jsonPath("/api/v1/stock-levels/%v/adjust", part["id"]), api.staff, map[string]any{"delta": 0})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "delta must not be zero")

	rec = api.do("POST", "/api/v1/stock-levels/9999/adjust", api.staff, map[string]any{"delta": 5})
	api.mustError(rec, http.StatusNotFound, "not_found", "part not found")
}

func TestStockListNestsPart(t *testing.T) {
	api := newTestAPI(t)
	api.createPart("AAA-1", 899, 4)
	part := api.createPart("BBB-2", 1299, 7)

	rec := api.do("GET", "/api/v1/stock-levels", api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Total != 2 {
		t.Fatalf("total: %+v", list)
	}
	nested, ok := list.Items[0]["part"].(map[string]any)
	if !ok || nested["sku"] != "AAA-1" {
		t.Fatalf("nested part: %v", list.Items[0])
	}

	rec = api.do("GET", jsonPath("/api/v1/stock-levels?part_id=%v", part["id"]), api.staff, nil)
	list = decodeBody[listBody[map[string]any]](t, rec)
	if list.Total != 1 || list.Items[0]["quantity_on_hand"] != float64(7) {
		t.Fatalf("part filter: %+v", list)
	}
}

func TestJobPartsOverAPI(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	part := api.createPart("IGN-1", 1299, 5)
	job := api.createJob(customer["id"], "Fix furnace", "")

	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 3,
	})
	api.mustStatus(rec, http.StatusCreated)
	jp := decodeBody[map[string]any](t, rec)
	if jp["unit_price_cents"] != float64(1299) || jp["quantity"] != float64(3) {
		t.Fatalf("job part: %v", jp)
	}

	// Overdraw fails atomically: stock still shows 2.
	rec = api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 3,
	})
	api.mustError(rec, http.StatusConflict, "insufficient_stock", "not enough stock on hand")
	rec = api.do("GET", jsonPath("/api/v1/stock-levels?part_id=%v", part["id"]), api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Items[0]["quantity_on_hand"] != float64(2) {
		t.Fatalf("stock after failed add: %v", list.Items[0])
	}

	rec = api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 0,
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "quantity must be greater than zero")

	rec = api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": 999, "quantity": 1,
	})
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "part_id must exist")

	// Removing the row returns the stock.
	rec = api.do("DELETE", jsonPath("/api/v1/job-parts/%v", jp["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusNoContent)
	rec = api.do("GET", jsonPath("/api/v1/stock-levels?part_id=%v", part["id"]), api.staff, nil)
	list = decodeBody[listBody[map[string]any]](t, rec)
	if list.Items[0]["quantity_on_hand"] != float64(5) {
		t.Fatalf("stock after removal: %v", list.Items[0])
	}
}

func TestDeleteJobOverAPIReturnsPartsToStock(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	part := api.createPart("IGN-1", 1299, 5)
	job := api.createJob(customer["id"], "Fix furnace", "")

	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 3,
	})
	api.mustStatus(rec, http.StatusCreated)

	// Deleting the job must be indistinguishable from deleting the job part
	// first: the stock comes all the way back.
	rec = api.do("DELETE", jsonPath("/api/v1/jobs/%v", job["id"]), api.staff, nil)
	api.mustStatus(rec, http.StatusNoContent)

	rec = api.do("GET", jsonPath("/api/v1/stock-levels?part_id=%v", part["id"]), api.staff, nil)
	list := decodeBody[listBody[map[string]any]](t, rec)
	if list.Items[0]["quantity_on_hand"] != float64(5) {
		t.Fatalf("stock after job delete: %v", list.Items[0])
	}
}

func TestDeleteUsedPartMessage(t *testing.T) {
	api := newTestAPI(t)
	customer := api.createCustomer("Harbor Coffee")
	part := api.createPart("IGN-1", 1299, 5)
	job := api.createJob(customer["id"], "Fix furnace", "")
	rec := api.do("POST", jsonPath("/api/v1/jobs/%v/parts", job["id"]), api.staff, map[string]any{
		"part_id": part["id"], "quantity": 1,
	})
	api.mustStatus(rec, http.StatusCreated)

	rec = api.do("DELETE", jsonPath("/api/v1/parts/%v", part["id"]), api.admin, nil)
	api.mustError(rec, http.StatusBadRequest, "validation_failed", "part has been used on jobs and cannot be deleted")
}
