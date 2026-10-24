package store

import "testing"

// The visible set is a restriction, not a hint. An empty visible set must
// return nothing: a caller that computed a visible set and got nothing
// back must see nothing.
func TestSearchFTSInEmptySetReturnsNothing(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	for i := 0; i < 5; i++ {
		s.UpsertNotes([]NoteRecord{{UID: string(rune('a' + i)), Path: "p", Title: "T", Body: "secretword"}})
	}
	out, err := s.SearchFTSIn("secretword", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("LEAK: an empty visible set returned %d notes", len(out))
	}
	out, err = s.SearchFTSIn("secretword", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("LEAK: a nil visible set returned %d notes", len(out))
	}
}

// The unrestricted path still works, and still returns BM25 order.
func TestSearchFTSUnrestrictedStillWorks(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{{UID: "a", Path: "p", Title: "T", Body: "secretword"}})
	out, err := s.SearchFTS("secretword")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != "a" {
		t.Fatalf("unrestricted search = %v, want [a]", out)
	}
}

// A restricted search returns only what was allowed.
func TestSearchFTSInRestricts(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{
		{UID: "a", Path: "p", Title: "T", Body: "secretword"},
		{UID: "b", Path: "p", Title: "T", Body: "secretword"},
	})
	out, err := s.SearchFTSIn("secretword", []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != "a" {
		t.Fatalf("restricted search = %v, want [a]", out)
	}
}
