package store

import (
	"errors"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func seedCustomer(t *testing.T, s *Store) *core.Customer {
	t.Helper()
	c := core.Customer{Name: "Harbor Coffee", ServiceAddress: "12 Pier Rd"}
	if err := s.CreateCustomer(&c); err != nil {
		t.Fatal(err)
	}
	return &c
}

func seedJob(t *testing.T, s *Store, customerID int64, title string) *core.Job {
	t.Helper()
	j := core.Job{CustomerID: customerID, Title: title}
	if err := s.CreateJob(&j); err != nil {
		t.Fatal(err)
	}
	return &j
}

func TestCreateJobForcesRequested(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	j := core.Job{CustomerID: c.ID, Title: "Fix furnace", Status: core.StatusCompleted}
	if err := s.CreateJob(&j); err != nil {
		t.Fatal(err)
	}
	if j.Status != core.StatusRequested {
		t.Fatalf("status = %s, want requested", j.Status)
	}
	if j.Priority != core.PriorityNormal {
		t.Fatalf("priority default = %s", j.Priority)
	}
}

func TestCreateJobUnknownCustomer(t *testing.T) {
	s := testStore(t)
	j := core.Job{CustomerID: 999, Title: "Fix furnace"}
	err := s.CreateJob(&j)
	var derr *core.Error
	// A dangling reference in the body is the caller's input being wrong,
	// not a missing resource: validation_failed, not not_found.
	if !errors.As(err, &derr) || derr.Code != core.CodeValidationFailed || derr.Fields["customer_id"] == "" {
		t.Fatalf("expected validation_failed on customer_id, got %v", err)
	}
}

func TestJobFilters(t *testing.T) {
	s := testStore(t)
	c1 := seedCustomer(t, s)
	c2 := seedCustomer(t, s)
	seedJob(t, s, c1.ID, "Furnace not igniting")
	seedJob(t, s, c1.ID, "Water heater leak")
	urgent := core.Job{CustomerID: c2.ID, Title: "Panel replacement", Priority: core.PriorityUrgent}
	if err := s.CreateJob(&urgent); err != nil {
		t.Fatal(err)
	}

	_, total, _ := s.ListJobs(JobFilter{CustomerID: &c1.ID}, Page{})
	if total != 2 {
		t.Errorf("customer filter: %d", total)
	}
	pr := core.PriorityUrgent
	_, total, _ = s.ListJobs(JobFilter{Priority: &pr}, Page{})
	if total != 1 {
		t.Errorf("priority filter: %d", total)
	}
	_, total, _ = s.ListJobs(JobFilter{Query: "heater"}, Page{})
	if total != 1 {
		t.Errorf("title filter: %d", total)
	}
	_, total, _ = s.ListJobs(JobFilter{Statuses: []core.JobStatus{core.StatusRequested}}, Page{})
	if total != 3 {
		t.Errorf("status filter: %d", total)
	}
}

func TestDeleteCustomerWithJobs(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	seedJob(t, s, c.ID, "Fix furnace")
	err := s.DeleteCustomer(c.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "customer has jobs and cannot be deleted" {
		t.Fatalf("expected guard, got %v", err)
	}
}

func TestJobStatusCounts(t *testing.T) {
	s := testStore(t)
	c := seedCustomer(t, s)
	seedJob(t, s, c.ID, "One")
	seedJob(t, s, c.ID, "Two")
	job := seedJob(t, s, c.ID, "Three")
	if _, err := s.TransitionJob(job.ID, core.StatusCancelled); err != nil {
		t.Fatal(err)
	}

	counts, err := s.JobStatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts[core.StatusRequested] != 2 || counts[core.StatusCancelled] != 1 {
		t.Fatalf("counts: %v", counts)
	}
	// Every status is present, zero or not, so the dashboard never misses a
	// column.
	if len(counts) != len(core.JobStatuses) {
		t.Fatalf("missing statuses: %v", counts)
	}
}
