package corpus

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// FTS is a ranked body index over a vault.
type FTS struct {
	db   *sql.DB
	path string
	// columns are the indexed columns, in schema order, excluding uid.
	columns []string
}

// OpenFTS opens (or creates) an FTS5 database at path.
func OpenFTS(path string) (*FTS, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &FTS{db: db, path: path}, nil
}

// DB exposes the underlying handle.
func (f *FTS) DB() *sql.DB { return f.db }

// Close closes the database.
func (f *FTS) Close() error { return f.db.Close() }

// The three schema shapes this spike measures.
//
// ADR-0030 gives `note_fts(uid UNINDEXED, body)`. ADR-0013 requires the
// note's title and aliases to be *boosted*, which BM25 can only do through
// column weights — so ADR-0030's schema has nowhere to put them. Each
// shape below is a different answer to that.

// SchemaBodyOnly is ADR-0030's schema verbatim. The title and aliases have
// to be prepended to the body text, so they cannot be boosted separately.
const SchemaBodyOnly = `
CREATE VIRTUAL TABLE IF NOT EXISTS note_fts USING fts5(
  uid UNINDEXED,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);`

// Schema adds columns for the title, the aliases and the tags.
const Schema = `
CREATE VIRTUAL TABLE IF NOT EXISTS note_fts USING fts5(
  uid UNINDEXED,
  title,
  aliases,
  tags,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);`

// SchemaHeadings adds a column for the note's headings.
const SchemaHeadings = `
CREATE VIRTUAL TABLE IF NOT EXISTS note_fts USING fts5(
  uid UNINDEXED,
  title,
  aliases,
  headings,
  tags,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);`

// SchemaColumns maps a schema to its indexed columns, in order.
func SchemaColumns(schema string) []string {
	switch schema {
	case SchemaBodyOnly:
		return []string{"body"}
	case Schema:
		return []string{"title", "aliases", "tags", "body"}
	case SchemaHeadings:
		return []string{"title", "aliases", "headings", "tags", "body"}
	}
	return nil
}

// Create makes the table.
func (f *FTS) Create(schema string) error {
	f.columns = SchemaColumns(schema)
	_, err := f.db.Exec(schema)
	return err
}

// Drop removes the table.
func (f *FTS) Drop() error {
	_, err := f.db.Exec(`DROP TABLE IF EXISTS note_fts;`)
	return err
}

// Doc is one document to index.
type Doc struct {
	UID      string
	Title    string
	Aliases  string
	Headings string
	Tags     string
	Body     string
}

// Insert writes the documents in one transaction. It returns the wall time
// spent.
func (f *FTS) Insert(docs []Doc) (time.Duration, error) {
	cols := f.columns
	placeholders := "?"
	for range cols {
		placeholders += ",?"
	}
	q := fmt.Sprintf(`INSERT INTO note_fts(uid,%s) VALUES(%s)`, strings.Join(cols, ","), placeholders)
	start := time.Now()
	tx, err := f.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(q)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, d := range docs {
		var args []any
		switch len(cols) {
		case 1:
			args = []any{d.UID, d.Body}
		case 4:
			args = []any{d.UID, d.Title, d.Aliases, d.Tags, d.Body}
		case 5:
			args = []any{d.UID, d.Title, d.Aliases, d.Headings, d.Tags, d.Body}
		default:
			return 0, fmt.Errorf("unsupported column count %d", len(cols))
		}
		if _, err := stmt.Exec(args...); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// Columns reports the indexed columns of the table, in schema order.
func (f *FTS) Columns() ([]string, error) {
	rows, err := f.db.Query(`SELECT name FROM pragma_table_info('note_fts')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name == "uid" {
			continue
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// Hit is one search result.
type Hit struct {
	UID   string
	Score float64 // bm25(); more negative is a better match
}

// Query runs a MATCH and returns the hits in BM25 order. The weights are
// per indexed column, in schema order.
func (f *FTS) Query(match string, limit int, weights []float64) ([]Hit, error) {
	if len(f.columns) == 0 {
		cols, err := f.Columns()
		if err != nil {
			return nil, err
		}
		f.columns = cols
	}
	if len(weights) != len(f.columns) {
		return nil, fmt.Errorf("want %d weights, got %d", len(f.columns), len(weights))
	}
	parts := make([]string, len(weights))
	for i, w := range weights {
		parts[i] = fmt.Sprintf("%.4f", w)
	}
	rows, err := f.db.Query(fmt.Sprintf(
		`SELECT uid, bm25(note_fts, %s) AS score FROM note_fts WHERE note_fts MATCH ? ORDER BY score LIMIT ?`,
		strings.Join(parts, ",")), match, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.UID, &h.Score); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// QueryUIDs runs a MATCH and returns only the uids, in rank order.
func (f *FTS) QueryUIDs(match string, limit int, weights []float64) ([]string, error) {
	hits, err := f.Query(match, limit, weights)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.UID)
	}
	return out, nil
}

// MatchQuery builds an FTS5 MATCH expression that matches any of the given
// terms (a disjunction), which is how a bare search box query is expressed.
func MatchQuery(terms []string) string {
	quoted := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		quoted = append(quoted, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " OR ")
}
