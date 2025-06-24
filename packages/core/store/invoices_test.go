package store

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/acme-dispatch/dispatch/core"
)

// completedJob walks a job to completed with one 1.5h weekday appointment for
// an 8000¢/h technician and 3 × 1299¢ parts — the shape of conformance case 4
// when the job is urgent.
func completedJob(t *testing.T, s *Store, priority core.Priority) *core.Job {
	t.Helper()
	c := seedCustomer(t, s)
	job := core.Job{CustomerID: c.ID, Title: "Furnace not igniting", Priority: priority}
	if err := s.CreateJob(&job); err != nil {
		t.Fatal(err)
	}
	tech := core.Technician{Name: "Dana Reyes", HourlyCostCents: 8000, IsActive: true}
	if err := s.CreateTechnician(&tech); err != nil {
		t.Fatal(err)
	}
	start := core.NewTime(time.Date(2025, 3, 12, 13, 0, 0, 0, time.UTC))
	end := core.NewTime(time.Date(2025, 3, 12, 14, 30, 0, 0, time.UTC))
	seedAppointment(t, s, job.ID, tech.ID, start, end)

	part := core.Part{SKU: fmt.Sprintf("IGN-4420-%d", job.ID), Name: "Hot surface igniter", UnitCostCents: 800, UnitPriceCents: 1299}
	if err := s.CreatePart(&part); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdjustStockLevel(part.ID, 10, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddJobPart(job.ID, part.ID, 3); err != nil {
		t.Fatal(err)
	}

	for _, st := range []core.JobStatus{core.StatusScheduled, core.StatusInProgress, core.StatusCompleted} {
		if _, err := s.TransitionJob(job.ID, st); err != nil {
			t.Fatal(err)
		}
	}
	fresh, _ := s.JobByID(job.ID)
	return fresh
}

func TestInvoiceMatchesCaseFour(t *testing.T) {
	s := testStore(t)
	job := completedJob(t, s, core.PriorityUrgent)

	rec, err := s.CreateInvoiceRecord(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.LaborCents != 19200 || rec.PartsCents != 3897 || rec.TotalCents != 25407 {
		t.Fatalf("case 4 totals: %d/%d/%d", rec.LaborCents, rec.PartsCents, rec.TotalCents)
	}
	if len(rec.LineItems) != 2 {
		t.Fatalf("line count: %d", len(rec.LineItems))
	}
	if rec.LineItems[0].Description != "Dana Reyes — 1.5h on 2025-03-12" {
		t.Errorf("labor line: %q", rec.LineItems[0].Description)
	}
	if rec.LineItems[1].Description != fmt.Sprintf("IGN-4420-%d — Hot surface igniter", job.ID) {
		t.Errorf("part line: %q", rec.LineItems[1].Description)
	}

	job, _ = s.JobByID(job.ID)
	if job.Status != core.StatusInvoiced {
		t.Fatalf("job not moved: %s", job.Status)
	}

	// Round-trip through the tables keeps everything.
	loaded, err := s.InvoiceRecordByID(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TotalCents != 25407 || len(loaded.LineItems) != 2 {
		t.Fatalf("reload: %+v", loaded)
	}
	if loaded.LineItems[0].Quantity != 1.5 || loaded.LineItems[1].Quantity != 3 {
		t.Fatalf("quantities: %v %v", loaded.LineItems[0].Quantity, loaded.LineItems[1].Quantity)
	}
}

func TestInvoiceGates(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	job := seedJob(t, s, c.ID, "Fix furnace")

	// Not completed yet: 409-shaped invalid_transition.
	_, err := s.CreateInvoiceRecord(job.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeInvalidTransition || derr.Message != "only a completed job can be invoiced" {
		t.Fatalf("uncompleted job: %v", err)
	}

	done := completedJob(t, s, core.PriorityNormal)
	if _, err := s.CreateInvoiceRecord(done.ID); err != nil {
		t.Fatal(err)
	}

	// Second invoice: the duplicate check wins and reads as validation_failed.
	_, err = s.CreateInvoiceRecord(done.ID)
	if !errors.As(err, &derr) || derr.Code != core.CodeValidationFailed || derr.Message != "job is already invoiced" {
		t.Fatalf("double invoice: %v", err)
	}
}

func TestInvoiceListFilters(t *testing.T) {
	s := testStore(t)
	a := completedJob(t, s, core.PriorityNormal)
	b := completedJob(t, s, core.PriorityNormal)
	if _, err := s.CreateInvoiceRecord(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInvoiceRecord(b.ID); err != nil {
		t.Fatal(err)
	}

	items, total, err := s.ListInvoiceRecords(InvoiceFilter{}, Page{})
	if err != nil || total != 2 {
		t.Fatalf("all: total=%d err=%v", total, err)
	}
	if len(items[0].LineItems) == 0 {
		t.Fatal("list items should include line items")
	}

	_, total, _ = s.ListInvoiceRecords(InvoiceFilter{JobID: &a.ID}, Page{})
	if total != 1 {
		t.Fatalf("job filter: %d", total)
	}
	_, total, _ = s.ListInvoiceRecords(InvoiceFilter{CustomerID: &b.CustomerID}, Page{})
	if total != 1 {
		t.Fatalf("customer filter: %d", total)
	}

	id, err := s.InvoiceRecordIDForJob(a.ID)
	if err != nil || id == nil {
		t.Fatalf("id for job: %v %v", id, err)
	}
	if id, _ := s.InvoiceRecordIDForJob(999); id != nil {
		t.Fatal("phantom invoice id")
	}
}

func TestInvoicedJobCannotBeDeleted(t *testing.T) {
	s := testStore(t)
	job := completedJob(t, s, core.PriorityNormal)
	if _, err := s.CreateInvoiceRecord(job.ID); err != nil {
		t.Fatal(err)
	}
	err := s.DeleteJob(job.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "an invoiced job cannot be deleted" {
		t.Fatalf("expected guard, got %v", err)
	}
}
