package store

import (
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func TestSeedShapeAndDeterminism(t *testing.T) {
	s := testStore(t)
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}

	counts, err := s.JobStatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	want := map[core.JobStatus]int{
		core.StatusRequested:  8,
		core.StatusScheduled:  8,
		core.StatusInProgress: 6,
		core.StatusCompleted:  6,
		core.StatusInvoiced:   7,
		core.StatusCancelled:  5,
	}
	for st, n := range want {
		if counts[st] != n {
			t.Errorf("%s: %d jobs, want %d", st, counts[st], n)
		}
	}

	_, customers, _ := s.ListCustomers(CustomerFilter{}, Page{})
	if customers != 20 {
		t.Errorf("customers: %d", customers)
	}
	_, techs, _ := s.ListTechnicians(TechnicianFilter{}, Page{})
	if techs != 8 {
		t.Errorf("technicians: %d", techs)
	}
	_, parts, _ := s.ListParts(PartFilter{}, Page{Size: 100})
	if parts != 25 {
		t.Errorf("parts: %d", parts)
	}
	invoices, invoiceTotal, err := s.ListInvoiceRecords(InvoiceFilter{}, Page{Size: 100})
	if err != nil || invoiceTotal != 7 {
		t.Fatalf("invoices: %d %v", invoiceTotal, err)
	}
	_, users, _ := s.ListUsers(UserFilter{}, Page{})
	if users != 2 {
		t.Errorf("users: %d", users)
	}

	// At least one invoice carries a weekend line and one job is urgent, so
	// both pricing branches show up in dev data.
	weekend := false
	for _, inv := range invoices {
		for _, line := range inv.LineItems {
			if line.Weekend {
				weekend = true
			}
		}
	}
	if !weekend {
		t.Error("no weekend line in any seeded invoice")
	}

	if admin, err := s.Authenticate("admin@acme-dispatch.example", "dispatch-admin"); err != nil || admin == nil || admin.Role != core.RoleAdmin {
		t.Fatalf("admin login: %v %v", admin, err)
	}

	// Re-seeding rebuilds the same world: same totals, same ids.
	firstTotals := make([]int64, 0, len(invoices))
	for _, inv := range invoices {
		firstTotals = append(firstTotals, inv.TotalCents)
	}
	if err := s.Seed(); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	again, n, _ := s.ListInvoiceRecords(InvoiceFilter{}, Page{Size: 100})
	if n != 7 {
		t.Fatalf("re-seed invoices: %d", n)
	}
	for i, inv := range again {
		if inv.TotalCents != firstTotals[i] {
			t.Errorf("invoice %d total changed across seeds: %d vs %d", i, inv.TotalCents, firstTotals[i])
		}
	}
}
