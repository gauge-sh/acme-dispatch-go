package store

import (
	"embed"
	"fmt"
	"sort"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies every migration under migrations/ that has not run yet, in
// filename order, each in its own transaction. Filenames are the version.
func (s *Store) Migrate() error {
	if _, err := s.exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`); err != nil {
		return err
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var done int
		if err := s.get(&done, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name); err != nil {
			return err
		}
		if done > 0 {
			continue
		}
		script, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		err = s.InTx(func(tx *Store) error {
			if _, err := tx.exec(string(script)); err != nil {
				return err
			}
			_, err := tx.exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// MigrationVersions lists applied migrations, oldest first.
func (s *Store) MigrationVersions() ([]string, error) {
	var versions []string
	err := s.selectAll(&versions, `SELECT version FROM schema_migrations ORDER BY version`)
	return versions, err
}
