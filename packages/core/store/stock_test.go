package store

import (
	"errors"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func seedPart(t *testing.T, s *Store, sku string, onHand int64) *core.Part {
	t.Helper()
	p := core.Part{SKU: sku, Name: "Part " + sku, UnitCostCents: 800, UnitPriceCents: 1299}
	if err := s.CreatePart(&p); err != nil {
		t.Fatal(err)
	}
	if onHand > 0 {
		if _, err := s.AdjustStockLevel(p.ID, onHand, nil); err != nil {
			t.Fatal(err)
		}
	}
	return &p
}

func TestPartCreationOpensStockLevel(t *testing.T) {
	s := testStore(t)
	p := seedPart(t, s, "IGN-4420", 0)
	level, err := s.StockLevelForPart(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if level.QuantityOnHand != 0 {
		t.Fatalf("new part should start at zero on hand, got %d", level.QuantityOnHand)
	}
}

func TestDuplicateSKU(t *testing.T) {
	s := testStore(t)
	seedPart(t, s, "IGN-4420", 0)
	dupe := core.Part{SKU: "IGN-4420", Name: "Another", UnitCostCents: 1, UnitPriceCents: 1}
	err := s.CreatePart(&dupe)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Fields["sku"] == "" {
		t.Fatalf("expected sku error, got %v", err)
	}
}

func TestAdjustStockLevel(t *testing.T) {
	s := testStore(t)
	p := seedPart(t, s, "FLT-16", 0)

	level, err := s.AdjustStockLevel(p.ID, 10, nil)
	if err != nil || level.QuantityOnHand != 10 {
		t.Fatalf("receive 10: %+v %v", level, err)
	}

	_, err = s.AdjustStockLevel(p.ID, -11, nil)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeInsufficientStock {
		t.Fatalf("overdraw: %v", err)
	}
	// The failed adjustment must not have written anything.
	fresh, _ := s.StockLevelForPart(p.ID)
	if fresh.QuantityOnHand != 10 {
		t.Fatalf("failed adjustment leaked: %d", fresh.QuantityOnHand)
	}

	_, err = s.AdjustStockLevel(p.ID, 0, nil)
	if !errors.As(err, &derr) || derr.Message != "delta must not be zero" {
		t.Fatalf("zero delta: %v", err)
	}

	_, err = s.AdjustStockLevel(999, 5, nil)
	if !errors.As(err, &derr) || derr.Code != core.CodeNotFound {
		t.Fatalf("unknown part: %v", err)
	}
}

func TestAdjustmentsAreAudited(t *testing.T) {
	s := testStore(t)
	p := seedPart(t, s, "FLT-16", 0)
	reason := "initial receiving"
	if _, err := s.AdjustStockLevel(p.ID, 10, &reason); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdjustStockLevel(p.ID, -3, nil); err != nil {
		t.Fatal(err)
	}

	entries, total, err := s.ListStockAdjustments(&p.ID, Page{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected 2 audit rows, got %d", total)
	}
	// Newest first.
	if entries[0].Delta != -3 || entries[0].QuantityAfter != 7 {
		t.Fatalf("latest entry: %+v", entries[0])
	}
	if entries[1].Reason == nil || *entries[1].Reason != "initial receiving" {
		t.Fatalf("reason lost: %+v", entries[1])
	}
}

func TestJobPartsConsumeAndReturnStock(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	p := seedPart(t, s, "IGN-4420", 5)

	jp, err := s.AddJobPart(job.ID, p.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	level, _ := s.StockLevelForPart(p.ID)
	if level.QuantityOnHand != 2 {
		t.Fatalf("stock after use: %d", level.QuantityOnHand)
	}

	// Not enough left: the whole request fails and nothing moves.
	_, err = s.AddJobPart(job.ID, p.ID, 3)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeInsufficientStock {
		t.Fatalf("expected insufficient_stock, got %v", err)
	}
	level, _ = s.StockLevelForPart(p.ID)
	if level.QuantityOnHand != 2 {
		t.Fatalf("failed request moved stock: %d", level.QuantityOnHand)
	}

	// Same part twice makes two rows.
	if _, err := s.AddJobPart(job.ID, p.ID, 2); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.JobPartsForJob(job.ID)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if err := s.RemoveJobPart(jp.ID); err != nil {
		t.Fatal(err)
	}
	level, _ = s.StockLevelForPart(p.ID)
	if level.QuantityOnHand != 3 {
		t.Fatalf("stock after return: %d", level.QuantityOnHand)
	}
}

func TestJobPartGates(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	p := seedPart(t, s, "IGN-4420", 5)

	_, err := s.AddJobPart(job.ID, p.ID, 0)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "quantity must be greater than zero" {
		t.Fatalf("zero quantity: %v", err)
	}

	dana := seedTech(t, s, "Dana")
	seedAppointment(t, s, job.ID, dana.ID, ts(2, 9), ts(2, 11))
	if _, err := s.TransitionJob(job.ID, core.StatusScheduled); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransitionJob(job.ID, core.StatusCancelled); err != nil {
		t.Fatal(err)
	}
	_, err = s.AddJobPart(job.ID, p.ID, 1)
	if !errors.As(err, &derr) || derr.Message != "parts cannot be added to a job that is cancelled" {
		t.Fatalf("cancelled gate: %v", err)
	}
}

func TestDeletePartInUse(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")
	p := seedPart(t, s, "IGN-4420", 5)
	if _, err := s.AddJobPart(job.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	err := s.DeletePart(p.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeValidationFailed {
		t.Fatalf("expected guard, got %v", err)
	}

	unused := seedPart(t, s, "FLT-16", 0)
	if err := s.DeletePart(unused.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StockLevelForPart(unused.ID); err == nil {
		t.Fatal("stock level should be gone with the part")
	}
}
