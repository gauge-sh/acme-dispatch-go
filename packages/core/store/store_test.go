package store

import (
	"path/filepath"
	"testing"
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
