package store

import (
	"errors"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func str(s string) *string { return &s }

func TestCustomerCRUD(t *testing.T) {
	s := testStore(t)

	c := core.Customer{Name: "Harbor Coffee", ServiceAddress: "12 Pier Rd", ContactEmail: str("ops@harbor.test")}
	if err := s.CreateCustomer(&c); err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 || c.CreatedAt.IsZero() {
		t.Fatalf("id/timestamps not set: %+v", c)
	}

	got, err := s.CustomerByID(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Harbor Coffee" || *got.ContactEmail != "ops@harbor.test" {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	got.Name = "Harbor Coffee Roasters"
	got.Phone = str("555-0100")
	if err := s.UpdateCustomer(got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CustomerByID(c.ID)
	if got.Name != "Harbor Coffee Roasters" || got.Phone == nil {
		t.Fatalf("update lost: %+v", got)
	}

	if err := s.DeleteCustomer(c.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.CustomerByID(c.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeNotFound {
		t.Fatalf("expected not_found, got %v", err)
	}
}

func TestCustomerListFilterAndPaging(t *testing.T) {
	s := testStore(t)
	names := []string{"Harbor Coffee", "Northside Dental", "Harbor Marina", "Cliffside Inn"}
	for _, n := range names {
		c := core.Customer{Name: n, ServiceAddress: "1 Main St"}
		if err := s.CreateCustomer(&c); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := s.ListCustomers(CustomerFilter{Query: "harbor"}, Page{Number: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("query filter: total=%d len=%d", total, len(items))
	}

	items, total, err = s.ListCustomers(CustomerFilter{}, Page{Number: 2, Size: 3})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(items) != 1 {
		t.Fatalf("page 2 of 3: total=%d len=%d", total, len(items))
	}

	// Address matches count too.
	_, total, _ = s.ListCustomers(CustomerFilter{Query: "main st"}, Page{})
	if total != 4 {
		t.Fatalf("address filter: total=%d", total)
	}
}

func TestCustomerValidationSurfaces(t *testing.T) {
	s := testStore(t)
	c := core.Customer{Name: "", ServiceAddress: ""}
	err := s.CreateCustomer(&c)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Code != core.CodeValidationFailed {
		t.Fatalf("expected validation_failed, got %v", err)
	}
	if derr.Fields["name"] == "" || derr.Fields["service_address"] == "" {
		t.Fatalf("fields missing: %v", derr.Fields)
	}
}
