// Package session owns the live session tier (ADR-0030, ADR-0038).
//
// A session is one sitting of a table. Its state lives in SQLite OUTSIDE
// the vault: it survives a server crash mid-session, it is rebuildable, and
// it is never synced into the vault. The vault owns durability of campaign
// content; the session tier owns ephemeral table state.
//
// This package is the ONE session store: one SQLite database, one schema,
// one code alphabet, one lifecycle guard. Seat tokens reuse the existing
// HMAC token family (issueToken/verifyToken) rather than a second scheme.
package session

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

func randRead(b []byte) (int, error) {
	return rand.Read(b)
}

// Status values.
const (
	StatusOpen   = "open"
	StatusClosed = "closed"
)

// Seat kinds (ADR-0038).
const (
	KindGM        = "gm"
	KindAccount   = "account"
	KindGuest     = "guest"
	KindSpectator = "spectator"
)

// Session is one live table sitting.
type Session struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	Campaign     string     `json:"campaign,omitempty"`
	Name         string     `json:"name,omitempty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	CloseBatchID string     `json:"close_batch_id,omitempty"`
	Seats        []Seat     `json:"seats,omitempty"`
	CodeAttempts int        `json:"code_attempts"`
}

// Seat is one person at the table.
type Seat struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Kind      string    `json:"kind"`
	Character string    `json:"character,omitempty"`
	Account   string    `json:"account,omitempty"`
	Claimed   bool      `json:"claimed"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is the SQLite-backed session store.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// Open opens (or creates) the session database at path. An empty path or
// ":memory:" uses an in-memory database.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		path = ":memory:"
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("session: mkdir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("session: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA synchronous=NORMAL;",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("session: pragma %q: %w", pragma, err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			campaign TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			closed_at TEXT,
			close_batch_id TEXT,
			code_attempts INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS seats (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			character_uid TEXT NOT NULL DEFAULT '',
			account_id TEXT NOT NULL DEFAULT '',
			claimed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return fmt.Errorf("session: migrate: %w", err)
		}
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	err := s.db.Close()
	s.db = nil
	return err
}

// DB returns the underlying *sql.DB.
func (s *Store) DB() *sql.DB { return s.db }

// --- code alphabet (single source) ---

// sessionCodeAlphabet is the unambiguous alphabet for session codes: no
// O/0, I/1, L. Six characters, distinct from the campaign code's eight.
const sessionCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// NewSessionCode generates a fresh six-character session code.
func NewSessionCode() (string, error) {
	b := make([]byte, 6)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	out := make([]byte, 6)
	for i := range out {
		out[i] = sessionCodeAlphabet[int(b[i])%len(sessionCodeAlphabet)]
	}
	return string(out), nil
}

// randRead is set in session_rand.go? define here via crypto/rand.
// We import crypto/rand in this file's other? Add import.

// CreateSession inserts a new open session.
func (s *Store) CreateSession(sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess.Status == "" {
		sess.Status = StatusOpen
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, code, campaign, name, status, created_at, code_attempts) VALUES (?,?,?,?,?,?,?)`,
		sess.ID, sess.Code, sess.Campaign, sess.Name, sess.Status, sess.CreatedAt.UTC().Format(time.RFC3339Nano), sess.CodeAttempts,
	)
	return err
}

// GetSession loads a session and its seats.
func (s *Store) GetSession(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT id, code, campaign, name, status, created_at, closed_at, close_batch_id, code_attempts FROM sessions WHERE id = ?`, id)
	var sess Session
	var created string
	var closed sql.NullString
	var batch sql.NullString
	if err := row.Scan(&sess.ID, &sess.Code, &sess.Campaign, &sess.Name, &sess.Status, &created, &closed, &batch, &sess.CodeAttempts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t, _ := time.Parse(time.RFC3339Nano, created)
	sess.CreatedAt = t
	if closed.Valid {
		ct, _ := time.Parse(time.RFC3339Nano, closed.String)
		sess.ClosedAt = &ct
	}
	if batch.Valid {
		sess.CloseBatchID = batch.String
	}
	seats, err := s.loadSeatsLocked(id)
	if err != nil {
		return nil, err
	}
	sess.Seats = seats
	return &sess, nil
}

func (s *Store) loadSeatsLocked(sessionID string) ([]Seat, error) {
	rows, err := s.db.Query(`SELECT id, session_id, kind, character_uid, account_id, claimed, created_at FROM seats WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Seat
	for rows.Next() {
		var st Seat
		var created string
		var claimed int
		if err := rows.Scan(&st.ID, &st.SessionID, &st.Kind, &st.Character, &st.Account, &claimed, &created); err != nil {
			return nil, err
		}
		st.Claimed = claimed != 0
		t, _ := time.Parse(time.RFC3339Nano, created)
		st.CreatedAt = t
		out = append(out, st)
	}
	return out, rows.Err()
}

// UpdateSession updates mutable fields.
func (s *Store) UpdateSession(sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var closed any
	if sess.ClosedAt != nil {
		closed = sess.ClosedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.Exec(
		`UPDATE sessions SET code=?, campaign=?, name=?, status=?, closed_at=?, close_batch_id=?, code_attempts=? WHERE id=?`,
		sess.Code, sess.Campaign, sess.Name, sess.Status, closed, nullStr(sess.CloseBatchID), sess.CodeAttempts, sess.ID,
	)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// AddSeat inserts a seat.
func (s *Store) AddSeat(seat Seat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seat.CreatedAt.IsZero() {
		seat.CreatedAt = time.Now().UTC()
	}
	claimed := 0
	if seat.Claimed {
		claimed = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO seats (id, session_id, kind, character_uid, account_id, claimed, created_at) VALUES (?,?,?,?,?,?,?)`,
		seat.ID, seat.SessionID, seat.Kind, seat.Character, seat.Account, claimed, seat.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// UpdateSeat updates a seat.
func (s *Store) UpdateSeat(seat Seat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	claimed := 0
	if seat.Claimed {
		claimed = 1
	}
	_, err := s.db.Exec(
		`UPDATE seats SET kind=?, character_uid=?, account_id=?, claimed=? WHERE id=?`,
		seat.Kind, seat.Character, seat.Account, claimed, seat.ID,
	)
	return err
}

// FindSeat returns a seat by id.
func (s *Store) FindSeat(sessionID, seatID string) (Seat, bool, error) {
	sess, err := s.GetSession(sessionID)
	if err != nil || sess == nil {
		return Seat{}, false, err
	}
	for _, st := range sess.Seats {
		if st.ID == seatID {
			return st, true, nil
		}
	}
	return Seat{}, false, nil
}

// CloseSession marks a session closed with batch id (idempotent at store level).
func (s *Store) CloseSession(id, batchID string) error {
	sess, err := s.GetSession(id)
	if err != nil || sess == nil {
		return err
	}
	if sess.Status == StatusClosed {
		// idempotent: keep existing batch id if present
		if sess.CloseBatchID == "" {
			sess.CloseBatchID = batchID
		}
		return s.UpdateSession(*sess)
	}
	now := time.Now().UTC()
	sess.Status = StatusClosed
	sess.ClosedAt = &now
	if sess.CloseBatchID == "" {
		sess.CloseBatchID = batchID
	}
	return s.UpdateSession(*sess)
}

// SetCodeAttempts updates code attempts count.
func (s *Store) SetCodeAttempts(id string, n int) error {
	sess, err := s.GetSession(id)
	if err != nil || sess == nil {
		return err
	}
	sess.CodeAttempts = n
	return s.UpdateSession(*sess)
}

// --- store registry (for seat-token validation across the request path) ---

var (
	regMu    sync.Mutex
	registry = map[string]*Store{}
)

// RegisterStore associates a token key with a session store so the request
// path can validate seat tokens without a store being threaded through every
// call.
func RegisterStore(tokenKey string, s *Store) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[tokenKey] = s
}

// StoreFor returns the store registered for a token key, if any.
func StoreFor(tokenKey string) *Store {
	regMu.Lock()
	defer regMu.Unlock()
	return registry[tokenKey]
}

// IsOpen reports whether session exists and is open.
func (s *Store) IsOpen(id string) bool {
	sess, err := s.GetSession(id)
	if err != nil || sess == nil {
		return false
	}
	return sess.Status == StatusOpen
}

// RotateCode replaces the session code.
func (s *Store) RotateCode(id, newCode string) error {
	sess, err := s.GetSession(id)
	if err != nil || sess == nil {
		return err
	}
	sess.Code = newCode
	return s.UpdateSession(*sess)
}

// JSON helpers for seats in responses are handled by callers.
var _ = json.Marshal
