// Package store holds the SQLite-derived caches (ADR-0030).
//
// The database is a cache: it can be rebuilt from the vault in one pass, and
// it lives outside the vault in the user's application-data directory. This
// package is only responsible for opening it with the right pragmas and
// checkpointing its WAL on close; the schema lives in the milestones that
// need it.
package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite cache database.
type Store struct {
	db        *sql.DB
	path      string
	cursorKey []byte
}

// Open opens (or creates) a cache database at path with the pragmas the
// project expects: WAL journal mode, a busy timeout, foreign keys, and
// NORMAL synchronous.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("store: empty path")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("store: mkdir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// Keep one connection so PRAGMAs and WAL behaviour are predictable.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA synchronous=NORMAL;",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("store: pragma %q: %w", pragma, err)
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: cursor key: %w", err)
	}
	return &Store{db: db, path: path, cursorKey: key}, nil
}

// DB returns the underlying *sql.DB.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the database path.
func (s *Store) Path() string { return s.path }

// Close checkpoints the WAL and closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	// TRUNCATE empties the WAL file so the -wal sidecar is gone after close.
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	err := s.db.Close()
	s.db = nil
	return err
}

// DataDir returns the application data directory, honouring XDG and an
// environment override so tests never write to a developer's home.
func DataDir() (string, error) {
	if v := os.Getenv("DEMITABLE_DATA_DIR"); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "demitable"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("store: home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "demitable"), nil
}
