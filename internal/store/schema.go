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
			meta TEXT,
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
	// Best-effort column additions for databases created by older versions.
	for _, col := range []string{"meta TEXT"} {
		_, _ = db.Exec(`ALTER TABLE notes ADD COLUMN ` + col)
	}
	return nil
}

// NoteRecord is a flat record for store insertion.
//
// Universal fields mirror the reserved frontmatter set (ADR-0040); Fields
// carries the GM's own (non-reserved) top-level fields, including a nested
// "data" object. Meta, when non-nil, overrides the derived metadata.
type NoteRecord struct {
	UID        string
	Path       string
	Title      string
	Name       string
	Type       string
	System     string
	Campaign   string
	Kind       string
	Player     string
	Workflow   string
	Visibility string
	Grants     []string
	Revealed   []string
	Discovered []string
	Aliases    []string
	Headings   []string
	Tags       []string
	Body       string
	Fields     map[string]any
	Data       any
	Meta       map[string]any
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
	notesStmt, err := tx.Prepare(`INSERT INTO notes(uid,path,title,meta,fts_rowid,updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(uid) DO UPDATE SET path=excluded.path,title=excluded.title,meta=excluded.meta,fts_rowid=excluded.fts_rowid,updated_at=excluded.updated_at`)
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
		meta, err := noteMetaJSON(n)
		if err != nil {
			return err
		}
		if _, err := notesStmt.Exec(n.UID, n.Path, n.Title, meta, rowids[i], now); err != nil {
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

// FTS runs an FTS5 MATCH query and returns uids in BM25 order (most
// relevant first). It is the unrestricted (GM) index.
func (s *Store) FTS(query string) ([]string, error) {
	return s.ftsQuery(query, nil)
}

// FTSIn is like FTS but restricted to the given uid set. A nil or empty
// allowed set means no results.
func (s *Store) FTSIn(query string, allowed []string) ([]string, error) {
	return s.ftsQuery(query, allowed)
}

func (s *Store) ftsQuery(query string, allowed []string) ([]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if allowed != nil && len(allowed) == 0 {
		return []string{}, nil
	}
	var rows *sql.Rows
	var err error
	if allowed == nil {
		rows, err = s.db.Query(`SELECT uid FROM note_fts WHERE note_fts MATCH ? ORDER BY bm25(note_fts)`, q)
	} else {
		placeholders := strings.Repeat("?,", len(allowed))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, 0, len(allowed)+1)
		args = append(args, q)
		for _, u := range allowed {
			args = append(args, u)
		}
		rows, err = s.db.Query(`SELECT uid FROM note_fts WHERE note_fts MATCH ? AND uid IN (`+placeholders+`) ORDER BY bm25(note_fts)`, args...)
	}
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
