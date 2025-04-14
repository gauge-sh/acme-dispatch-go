package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/acme-dispatch/dispatch/core"
)

// Store wraps the SQLite handle. Inside InTx the same methods run against the
// transaction, so callers never juggle two APIs.
type Store struct {
	db  *sqlx.DB
	ext sqlx.Ext
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer; a single pooled connection keeps SQLITE_BUSY
	// out of the picture entirely at the scale this tool runs at.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db, ext: db}
	if err := s.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) InTx(fn func(*Store) error) error {
	if s.db == nil {
		return fn(s) // already inside a transaction
	}
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	if err := fn(&Store{ext: tx}); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) get(dest any, query string, args ...any) error {
	return sqlx.Get(s.ext, dest, query, args...)
}

func (s *Store) selectAll(dest any, query string, args ...any) error {
	return sqlx.Select(s.ext, dest, query, args...)
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.ext.Exec(query, args...)
}

func noRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// Page is a validated pagination window; the API and web layers construct it
// from query parameters.
type Page struct {
	Number int
	Size   int
}

func (p Page) limit() int {
	if p.Size <= 0 {
		return 20
	}
	return p.Size
}

func (p Page) offset() int {
	if p.Number <= 1 {
		return 0
	}
	return (p.Number - 1) * p.limit()
}

func touch(created *core.Time, updated *core.Time) {
	if created.IsZero() {
		*created = core.Now()
	}
	if updated.IsZero() {
		*updated = *created
	}
}
