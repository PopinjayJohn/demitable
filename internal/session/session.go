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

	"github.com/PopinjayJohn/demitable/internal/dice"
	"github.com/PopinjayJohn/demitable/internal/transcribe"

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

// ErrSessionClosed is returned by store methods that mutate a closed session.
var ErrSessionClosed = errors.New("session_closed")

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

// ScratchEntry is one disposable, session-tier note (ADR-0048). It lives in
// SQLite only, never in the vault or the journal, and is purged on close.
type ScratchEntry struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	SeatID    string    `json:"seat_id,omitempty"`
	Author    string    `json:"author,omitempty"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	NoteUID   string    `json:"note_uid,omitempty"`
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
		`CREATE TABLE IF NOT EXISTS scratch (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			seat_id TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			note_uid TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS encounters (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			state TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS rolls (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			encounter_id TEXT NOT NULL DEFAULT '',
			roller TEXT NOT NULL DEFAULT '',
			workflow TEXT NOT NULL DEFAULT '',
			expression TEXT NOT NULL DEFAULT '',
			shared INTEGER NOT NULL DEFAULT 1,
			timestamp TEXT NOT NULL,
			payload TEXT NOT NULL DEFAULT '{}',
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS transcript_segments (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			text TEXT NOT NULL DEFAULT '',
			start REAL NOT NULL DEFAULT 0,
			end REAL NOT NULL DEFAULT 0,
			speaker TEXT NOT NULL DEFAULT '',
			reviewed INTEGER NOT NULL DEFAULT 0,
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
// O/0, I/1, L. Six characters. The alphabet is shared with the campaign
// join code; disjointness between a session code and a campaign code rests
// on code length (6 vs 8), the endpoint that consumes it, and the store
// that validates it — not on disjoint alphabets.
const sessionCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

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

// UpdateSession updates mutable fields. Closed sessions reject updates
// (ErrSessionClosed); CloseSession is the only path that may write closed
// state, and it bypasses this guard via updateSessionUnlocked.
func (s *Store) UpdateSession(sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sess.ID).Scan(&status); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	} else if status == StatusClosed {
		return ErrSessionClosed
	}
	return s.updateSessionUnlocked(sess)
}

// updateSessionUnlocked performs the UPDATE without locking or the closed-session
// guard. Caller must hold s.mu.
func (s *Store) updateSessionUnlocked(sess Session) error {
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
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, seat.SessionID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session: not found: %w", err)
		}
		return err
	}
	if status == StatusClosed {
		return ErrSessionClosed
	}
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
	var sessionID string
	if err := s.db.QueryRow(`SELECT session_id FROM seats WHERE id = ?`, seat.ID).Scan(&sessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session: seat not found: %w", err)
		}
		return err
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err != nil {
		return err
	}
	if status == StatusClosed {
		return ErrSessionClosed
	}
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

// ClaimSeat atomically claims one unclaimed seat in a session.
//
// If accountID is non-empty, only a seat pre-added for that account is
// claimed; otherwise any unclaimed seat is claimed. The claim is a single
// transaction (BEGIN IMMEDIATE) so concurrent callers cannot both win the
// same seat. Returns the claimed seat and true on success; (zero, false, nil)
// when no suitable unclaimed seat exists; an error (including
// ErrSessionClosed) on failure.
func (s *Store) ClaimSeat(sessionID, accountID string) (Seat, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec("BEGIN IMMEDIATE"); err != nil {
		return Seat{}, false, err
	}
	rollback := func() { _, _ = s.db.Exec("ROLLBACK") }

	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err != nil {
		rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return Seat{}, false, err
		}
		return Seat{}, false, err
	}
	if status == StatusClosed {
		rollback()
		return Seat{}, false, ErrSessionClosed
	}

	q := `SELECT id, session_id, kind, character_uid, account_id, claimed, created_at
		FROM seats WHERE session_id = ? AND claimed = 0`
	args := []any{sessionID}
	if accountID != "" {
		q += ` AND account_id = ?`
		args = append(args, accountID)
	}
	q += ` ORDER BY created_at, id LIMIT 1`

	row := s.db.QueryRow(q, args...)
	var st Seat
	var created string
	var claimed int
	if err := row.Scan(&st.ID, &st.SessionID, &st.Kind, &st.Character, &st.Account, &claimed, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			rollback()
			return Seat{}, false, nil
		}
		rollback()
		return Seat{}, false, err
	}
	st.Claimed = false

	// Atomic claim: only succeeds if still unclaimed.
	res, err := s.db.Exec(`UPDATE seats SET claimed = 1 WHERE id = ? AND claimed = 0`, st.ID)
	if err != nil {
		rollback()
		return Seat{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		rollback()
		return Seat{}, false, nil
	}
	if _, err := s.db.Exec("COMMIT"); err != nil {
		_, _ = s.db.Exec("ROLLBACK")
		return Seat{}, false, err
	}
	st.Claimed = true
	t, _ := time.Parse(time.RFC3339Nano, created)
	st.CreatedAt = t
	return st, true, nil
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
		s.mu.Lock()
		_, _ = s.db.Exec(`DELETE FROM scratch WHERE session_id = ?`, id)
		err = s.updateSessionUnlocked(*sess)
		s.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	sess.Status = StatusClosed
	sess.ClosedAt = &now
	if sess.CloseBatchID == "" {
		sess.CloseBatchID = batchID
	}
	// Scratch is disposable: purge all rows for this session on close.
	s.mu.Lock()
	_, _ = s.db.Exec(`DELETE FROM scratch WHERE session_id = ?`, id)
	err = s.updateSessionUnlocked(*sess)
	s.mu.Unlock()
	return err
}

// SetCodeAttempts updates code attempts count.
func (s *Store) SetCodeAttempts(id string, n int) error {
	sess, err := s.GetSession(id)
	if err != nil || sess == nil {
		if err == nil {
			return sql.ErrNoRows
		}
		return err
	}
	if sess.Status == StatusClosed {
		return ErrSessionClosed
	}
	sess.CodeAttempts = n
	return s.UpdateSession(*sess)
}

// AppendScratch inserts a scratch entry. Closed sessions reject writes.
func (s *Store) AppendScratch(e ScratchEntry) (ScratchEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, e.SessionID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ScratchEntry{}, fmt.Errorf("session: not found: %w", err)
		}
		return ScratchEntry{}, err
	}
	if status == StatusClosed {
		return ScratchEntry{}, ErrSessionClosed
	}
	if e.ID == "" {
		id := ""
		if id == "" {
			id = fmt.Sprintf("scratch-%d", time.Now().UnixNano())
		}
		e.ID = id
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	tags, _ := json.Marshal(e.Tags)
	if e.Tags == nil {
		tags = []byte("[]")
	}
	_, err := s.db.Exec(
		`INSERT INTO scratch (id, session_id, seat_id, author, content, tags, note_uid, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, e.SessionID, e.SeatID, e.Author, e.Content, string(tags), e.NoteUID, e.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return ScratchEntry{}, err
	}
	return e, nil
}

// ListScratch returns all scratch rows for a session, oldest first.
func (s *Store) ListScratch(sessionID string) ([]ScratchEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id, session_id, seat_id, author, content, tags, note_uid, created_at FROM scratch WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScratchEntry
	for rows.Next() {
		var e ScratchEntry
		var created string
		var tags string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.SeatID, &e.Author, &e.Content, &tags, &e.NoteUID, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tags), &e.Tags)
		t, _ := time.Parse(time.RFC3339Nano, created)
		e.CreatedAt = t
		out = append(out, e)
	}
	if out == nil {
		out = []ScratchEntry{}
	}
	return out, rows.Err()
}

// PurgeScratch deletes all scratch rows for a session.
func (s *Store) PurgeScratch(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM scratch WHERE session_id = ?`, sessionID)
	return err
}

// IsClosed reports whether the session is closed.
func (s *Store) IsClosed(id string) bool {
	sess, err := s.GetSession(id)
	if err != nil || sess == nil {
		return true
	}
	return sess.Status == StatusClosed
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
		if err == nil {
			return sql.ErrNoRows
		}
		return err
	}
	if sess.Status == StatusClosed {
		return ErrSessionClosed
	}
	sess.Code = newCode
	return s.UpdateSession(*sess)
}

// JSON helpers for seats in responses are handled by callers.
var _ = json.Marshal

// --- encounter live state (F2b) ---------------------------------------------

// CreateEncounter stores live encounter state for a session. Closed sessions
// reject writes.
func (s *Store) CreateEncounter(sessionID, encounterID string, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session: not found: %w", err)
		}
		return err
	}
	if status == StatusClosed {
		return ErrSessionClosed
	}
	if len(state) == 0 {
		state = []byte("{}")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO encounters (id, session_id, state, created_at, updated_at) VALUES (?,?,?,?,?)`,
		encounterID, sessionID, string(state), now, now,
	)
	if err != nil {
		// upsert on conflict
		_, err = s.db.Exec(
			`UPDATE encounters SET session_id=?, state=?, updated_at=? WHERE id=?`,
			sessionID, string(state), now, encounterID,
		)
	}
	return err
}

// GetEncounterState returns the stored live state for an encounter.
func (s *Store) GetEncounterState(sessionID, encounterID string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st string
	var sid string
	err := s.db.QueryRow(`SELECT session_id, state FROM encounters WHERE id = ?`, encounterID).Scan(&sid, &st)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if sid != sessionID && sessionID != "" {
		// still return if session matches? enforce session scoping
		if sid != sessionID {
			return nil, false, nil
		}
	}
	return []byte(st), true, nil
}

// UpdateEncounterState updates live state. Closed sessions reject.
func (s *Store) UpdateEncounterState(sessionID, encounterID string, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sid, status string
	err := s.db.QueryRow(`SELECT session_id FROM encounters WHERE id = ?`, encounterID).Scan(&sid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if sessionID == "" {
				return fmt.Errorf("session: not found: %w", sql.ErrNoRows)
			}
			if err2 := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err2 != nil {
				if errors.Is(err2, sql.ErrNoRows) {
					return fmt.Errorf("session: not found: %w", err2)
				}
				return err2
			}
			if status == StatusClosed {
				return ErrSessionClosed
			}
			return fmt.Errorf("encounter not found")
		}
		return err
	}
	if sid == "" {
		sid = sessionID
	}
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sid).Scan(&status); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	} else if status == StatusClosed {
		return ErrSessionClosed
	}
	if len(state) == 0 {
		state = []byte("{}")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE encounters SET state=?, updated_at=? WHERE id=? AND session_id=?`, string(state), now, encounterID, sid)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("encounter not found")
	}
	return nil
}

// ListEncounters returns encounter ids for a session.
func (s *Store) ListEncounters(sessionID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id FROM encounters WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

// DeleteEncounter removes an encounter live state row.
func (s *Store) DeleteEncounter(sessionID, encounterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID != "" {
		var status string
		if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("session: not found: %w", err)
			}
			return err
		}
		if status == StatusClosed {
			return ErrSessionClosed
		}
	}
	_, err := s.db.Exec(`DELETE FROM encounters WHERE id = ? AND session_id = ?`, encounterID, sessionID)
	return err
}

// --- rolls (session tier) ---------------------------------------------------

// AppendRoll persists a roll record to the session tier.
func (s *Store) AppendRoll(sessionID, encounterID string, rec dice.RollRecord) (dice.RollRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rec, fmt.Errorf("session: not found: %w", err)
		}
		return rec, err
	}
	if status == StatusClosed {
		return rec, ErrSessionClosed
	}
	if rec.ID == "" {
		rec.ID = fmt.Sprintf("roll-%d", time.Now().UnixNano())
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}
	rec.SessionID = sessionID
	rec.EncounterID = encounterID
	rec.SyncAliases()
	payload, _ := json.Marshal(rec)
	shared := 0
	if rec.Shared {
		shared = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO rolls (id, session_id, encounter_id, roller, workflow, expression, shared, timestamp, payload) VALUES (?,?,?,?,?,?,?,?,?)`,
		rec.ID, sessionID, encounterID, rec.Roller, rec.Workflow, rec.Expression, shared, rec.Timestamp.UTC().Format(time.RFC3339Nano), string(payload),
	)
	if err != nil {
		return rec, err
	}
	return rec, nil
}

// ListRolls returns rolls for a session, optionally filtered by encounter.
func (s *Store) ListRolls(sessionID, encounterID string) ([]dice.RollRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := `SELECT payload, shared FROM rolls WHERE session_id = ?`
	args := []any{sessionID}
	if encounterID != "" {
		q += ` AND encounter_id = ?`
		args = append(args, encounterID)
	}
	q += ` ORDER BY timestamp, id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dice.RollRecord
	for rows.Next() {
		var payload string
		var shared int
		if err := rows.Scan(&payload, &shared); err != nil {
			return nil, err
		}
		var rec dice.RollRecord
		if err := json.Unmarshal([]byte(payload), &rec); err != nil {
			continue
		}
		rec.Shared = shared != 0
		out = append(out, rec)
	}
	if out == nil {
		out = []dice.RollRecord{}
	}
	return out, rows.Err()
}

// --- transcript segments (P2, session tier) --------------------------------

// TranscriptSegment is an alias for transcribe.Segment (one segment path).
type TranscriptSegment = transcribe.Segment

// AppendTranscriptSegment stores one segment for a session. Closed sessions
// reject writes.
func (s *Store) AppendTranscriptSegment(sessionID string, seg transcribe.Segment) (transcribe.Segment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, sessionID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return seg, fmt.Errorf("session: not found: %w", err)
		}
		return seg, err
	}
	if status == StatusClosed {
		return seg, ErrSessionClosed
	}
	if seg.ID == "" {
		seg.ID = fmt.Sprintf("seg-%d", time.Now().UnixNano())
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO transcript_segments (id, session_id, text, start, end, speaker, reviewed, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		seg.ID, sessionID, seg.Text, seg.Start, seg.End, seg.Speaker, 0, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return seg, err
	}
	return seg, nil
}

// ListTranscriptSegments returns all segments for a session, oldest first.
func (s *Store) ListTranscriptSegments(sessionID string) ([]transcribe.Segment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id, text, start, end, speaker, reviewed FROM transcript_segments WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []transcribe.Segment
	for rows.Next() {
		var seg transcribe.Segment
		var reviewed int
		if err := rows.Scan(&seg.ID, &seg.Text, &seg.Start, &seg.End, &seg.Speaker, &reviewed); err != nil {
			return nil, err
		}
		out = append(out, seg)
	}
	if out == nil {
		out = []transcribe.Segment{}
	}
	return out, rows.Err()
}

// DeleteTranscriptSegments removes all segments for a session.
func (s *Store) DeleteTranscriptSegments(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM transcript_segments WHERE session_id = ?`, sessionID)
	return err
}
