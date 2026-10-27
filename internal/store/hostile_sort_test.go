package store

import (
	"errors"
	"testing"
)

func TestHostileSortRejected(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{{UID: "a", Path: "p", Title: "T", Body: "body"}})

	malicious := []string{
		`'`,
		`uid') OR ('1'=='1`,
		`uid; DROP TABLE notes;--`,
		`json_extract(meta,'$.uid')`,
	}
	for _, sort := range malicious {
		_, err := s.Query(QueryRequest{Sort: sort})
		if err == nil || !errors.Is(err, ErrParse) {
			t.Fatalf("malicious sort %q not rejected with ErrParse: %v", sort, err)
		}
	}
}
