package store

import (
	"path/filepath"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrationsApplyOnceAndInOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.MigrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("no migrations applied")
	}
	for i := 1; i < len(first); i++ {
		if first[i] <= first[i-1] {
			t.Errorf("migrations out of order: %s before %s", first[i-1], first[i])
		}
	}
	s.Close()

	// Reopening must be a no-op, not a re-run.
	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	second, err := s.MigrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != len(first) {
		t.Errorf("reopen applied %d migrations, had %d", len(second), len(first))
	}
}

func TestDataSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := core.Customer{Name: "Persistent Co", ServiceAddress: "1 Disk Ln"}
	if err := s.CreateCustomer(&c); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.CustomerByID(c.ID)
	if err != nil || got.Name != "Persistent Co" {
		t.Fatalf("reopen lost data: %v %v", got, err)
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	s := testStore(t)
	// A raw insert that dodges the store's checks still hits the schema.
	_, err := s.exec(`INSERT INTO appointments (job_id, technician_id, scheduled_start, scheduled_end, created_at, updated_at)
		VALUES (999, 999, '2025-01-01T09:00:00Z', '2025-01-01T10:00:00Z', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("foreign keys are off")
	}
}
