// Package store owns all SQLite access: connection setup, migrations and the
// typed repositories used by the rest of the engine. SQLite is the single
// source of truth for read state and cached content.
package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"

	feedclaw "github.com/marceloalmeidadev/feedclaw"

	_ "modernc.org/sqlite" // CGO-free SQLite driver
)

// Store wraps the database handle and exposes the repositories.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path, enables the
// pragmas we rely on and runs pending migrations. Use ":memory:" for tests.
func Open(path string) (*Store, error) {
	// Foreign keys must be enabled per-connection; busy_timeout avoids
	// spurious "database is locked" under the fetcher's concurrent writers.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single writer connection sidesteps SQLITE_BUSY on an in-process DB
	// while readers still use WAL. The fetcher serializes writes anyway.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB exposes the underlying handle for packages that need raw access (kept
// small on purpose; prefer the repository methods).
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// migrate applies every embedded migration that has not run yet, tracked in a
// schema_migrations table. Migrations are plain SQL applied in filename order.
func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(feedclaw.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := s.migrationApplied(name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := s.applyMigration(name); err != nil {
			return err
		}
	}
	return nil
}

// migrationApplied reports whether the named migration has already run.
func (s *Store) migrationApplied(name string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM schema_migrations WHERE name = ?`, name).Scan(&exists)
	switch err {
	case nil:
		return true, nil
	case sql.ErrNoRows:
		return false, nil
	default:
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
}

// applyMigration runs one migration file and records it, atomically.
func (s *Store) applyMigration(name string) error {
	body, err := fs.ReadFile(feedclaw.Migrations, "migrations/"+name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(string(body)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}
