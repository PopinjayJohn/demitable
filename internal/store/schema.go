package store

import (
	"database/sql"
	"fmt"
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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Pre-load existing fts rowids in one query to avoid per-row lookups.
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
	for _, n := range notes {
		if rid, ok := oldRowids[n.UID]; ok {
			if _, err := tx.Exec(`DELETE FROM note_fts WHERE rowid = ?`, rid); err != nil {
				return err
			}
		}
		res, err := tx.Exec(`INSERT INTO note_fts(uid,title,aliases,headings,tags,body) VALUES(?,?,?,?,?,?)`,
			n.UID, n.Title, join(n.Aliases), join(n.Headings), join(n.Tags), n.Body)
		if err != nil {
			return err
		}
		rid, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO notes(uid,path,title,fts_rowid,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(uid) DO UPDATE SET path=excluded.path,title=excluded.title,fts_rowid=excluded.fts_rowid,updated_at=excluded.updated_at`,
			n.UID, n.Path, n.Title, rid, 0); err != nil {
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

// SearchFTS runs a MATCH query and returns uids.
func (s *Store) SearchFTS(query string) ([]string, error) {
	rows, err := s.db.Query(`SELECT uid FROM note_fts WHERE note_fts MATCH ?`, query)
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
	return out, rows.Err()
}
