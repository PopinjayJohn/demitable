package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Migrate creates the notes table and the FTS5 index.
func (s *Store) Migrate() error {
	db := s.db
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			uid TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			title TEXT,
			fts_rowid INTEGER,
			updated_at INTEGER
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS note_fts USING fts5(
			uid UNINDEXED,
			title,
			aliases,
			headings,
			tags,
			body,
			tokenize = 'unicode61 remove_diacritics 2'
		);`,
	}
	for _, st := range stmts {
		if _, err := db.Exec(st); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

// NoteRecord is a flat record for store insertion.
type NoteRecord struct {
	UID      string
	Path     string
	Title    string
	Aliases  []string
	Headings []string
	Tags     []string
	Body     string
}

// UpsertNote inserts or replaces a note row and its FTS entry.
func (s *Store) UpsertNote(uid, path, title string, aliases, headings, tags []string, body string) error {
	return s.UpsertNotes([]NoteRecord{{UID: uid, Path: path, Title: title, Aliases: aliases, Headings: headings, Tags: tags, Body: body}})
}

// UpsertNotes inserts or replaces many notes in one transaction.
func (s *Store) UpsertNotes(notes []NoteRecord) error {
	if len(notes) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Pre-load existing fts rowids in one query.
	oldRowids := map[string]int64{}
	rows, _ := tx.Query(`SELECT uid, fts_rowid FROM notes`)
	if rows != nil {
		for rows.Next() {
			var uid string
			var rid sql.NullInt64
			rows.Scan(&uid, &rid)
			if rid.Valid {
				oldRowids[uid] = rid.Int64
			}
		}
		rows.Close()
	}

	// Batch-delete old FTS rows (replacements only).
	var toDelete []interface{}
	for _, n := range notes {
		if rid, ok := oldRowids[n.UID]; ok {
			toDelete = append(toDelete, rid)
		}
	}
	if len(toDelete) > 0 {
		placeholders := strings.Repeat("?,", len(toDelete))
		placeholders = placeholders[:len(placeholders)-1]
		if _, err := tx.Exec(`DELETE FROM note_fts WHERE rowid IN (`+placeholders+`)`, toDelete...); err != nil {
			return err
		}
	}

	// Prepare both statements once per transaction: tx.Exec prepares and
	// finalises a statement on every call, which is the per-row cost that
	// pushed the insert over the criterion.
	ftsStmt, err := tx.Prepare(`INSERT INTO note_fts(uid,title,aliases,headings,tags,body) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ftsStmt.Close()
	notesStmt, err := tx.Prepare(`INSERT INTO notes(uid,path,title,fts_rowid,updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(uid) DO UPDATE SET path=excluded.path,title=excluded.title,fts_rowid=excluded.fts_rowid,updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer notesStmt.Close()

	rowids := make([]int64, len(notes))
	for i, n := range notes {
		res, err := ftsStmt.Exec(n.UID, n.Title, join(n.Aliases), join(n.Headings), join(n.Tags), n.Body)
		if err != nil {
			return err
		}
		rid, err := res.LastInsertId()
		if err != nil {
			return err
		}
		rowids[i] = rid
	}

	// Multi-row upsert into notes table via the prepared statement, per row.
	now := time.Now().Unix()
	for i, n := range notes {
		if _, err := notesStmt.Exec(n.UID, n.Path, n.Title, rowids[i], now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

// SearchFTS runs a MATCH query and returns uids in BM25 order (most
// relevant first). FTS5's bm25() returns a negative score where more
// negative is better, so ascending order is the right one.
// SearchFTS is like SearchFTSIn with no restriction. **It is for the GM
// only.** A non-GM caller must use SearchFTSIn with its visible set,
// because an unrestricted search ranks against every note in the index -
// including the GM-only ones, which is the side channel ADR-0054
// measured.
func (s *Store) SearchFTS(query string) ([]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT uid FROM note_fts WHERE note_fts MATCH ? ORDER BY bm25(note_fts)`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

// SearchFTSIn is like SearchFTS but restricted to the given uid set, so a
// caller can rank only within a visible set (ADR-0054's partition).
//
// A nil or empty allowed set means **no results**, not "no restriction":
// a caller that computed a visible set and got nothing back must see
// nothing, or the restriction is not a restriction. Use SearchFTS
// explicitly when the unrestricted index is what you want.
func (s *Store) SearchFTSIn(query string, allowed []string) ([]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if len(allowed) == 0 {
		// The visible set is empty, so nothing is visible.
		return []string{}, nil
	}
	placeholders := strings.Repeat("?,", len(allowed))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(allowed)+1)
	args = append(args, q)
	for _, u := range allowed {
		args = append(args, u)
	}
	rows, err := s.db.Query(`SELECT uid FROM note_fts WHERE note_fts MATCH ? AND uid IN (`+placeholders+`) ORDER BY bm25(note_fts)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}
