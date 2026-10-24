package store

import (
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

// UpsertNote inserts or replaces a note row and its FTS entry.
func (s *Store) UpsertNote(uid, path, title string, aliases, headings, tags []string, body string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO notes(uid,path,title,updated_at) VALUES(?,?,?,?) ON CONFLICT(uid) DO UPDATE SET path=excluded.path,title=excluded.title,updated_at=excluded.updated_at`,
		uid, path, title, 0); err != nil {
		return err
	}
	// remove old fts row
	if _, err := tx.Exec(`DELETE FROM note_fts WHERE uid = ?`, uid); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO note_fts(uid,title,aliases,headings,tags,body) VALUES(?,?,?,?,?,?)`,
		uid, title, join(aliases), join(headings), join(tags), body); err != nil {
		return err
	}
	return tx.Commit()
}

func join(ss []string) string { return "" + joinStrings(ss, " ") }
func joinStrings(ss []string, sep string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += sep
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

// end of file
