package store

import "testing"

// The visible set is a restriction, not a hint. An empty visible set must
// return nothing: a caller that computed a visible set and got nothing
// back must see nothing.
// Canonical nil/empty convention for QueryRequest.Allowed (and FTSIn):
// nil means unrestricted (GM path); a non-nil empty slice means no results.
func TestQueryEmptyAllowedSetReturnsNothing(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	for i := 0; i < 5; i++ {
		s.UpsertNotes([]NoteRecord{{UID: string(rune('a' + i)), Path: "p", Title: "T", Body: "secretword"}})
	}
	res, err := s.Query(QueryRequest{Query: "body contains \"secretword\"", Allowed: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UIDs) != 0 {
		t.Errorf("LEAK: an empty visible set returned %d notes", len(res.UIDs))
	}
	res, err = s.Query(QueryRequest{Query: "body contains \"secretword\"", Allowed: nil})
	if err != nil {
		t.Fatal(err)
	}
	// nil allowed is unrestricted (GM path); all 5 match.
	if len(res.UIDs) != 5 {
		t.Errorf("unrestricted returned %d, want 5", len(res.UIDs))
	}
	// Empty non-nil allowed still nothing.
	res, err = s.Query(QueryRequest{Allowed: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UIDs) != 0 {
		t.Errorf("LEAK: empty allowed returned %d", len(res.UIDs))
	}
}

// The unrestricted path still works.
func TestQueryUnrestrictedStillWorks(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{{UID: "a", Path: "p", Title: "T", Body: "secretword"}})
	res, err := s.Query(QueryRequest{Query: "body contains \"secretword\""})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UIDs) != 1 || res.UIDs[0] != "a" {
		t.Fatalf("unrestricted search = %v, want [a]", res.UIDs)
	}
}

// A restricted search returns only what was allowed.
func TestQueryAllowedSetRestricts(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{
		{UID: "a", Path: "p", Title: "T", Body: "secretword"},
		{UID: "b", Path: "p", Title: "T", Body: "secretword"},
	})
	res, err := s.Query(QueryRequest{Query: "body contains \"secretword\"", Allowed: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UIDs) != 1 || res.UIDs[0] != "a" {
		t.Fatalf("restricted search = %v, want [a]", res.UIDs)
	}
}
