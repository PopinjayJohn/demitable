package store

import "testing"

func TestOracleHiddenNoteDoesNotAffectPlayerResults(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	// Player-visible notes.
	s.UpsertNotes([]NoteRecord{
		{UID: "v1", Path: "p", Title: "T", Body: "xylophone visible", Visibility: "all"},
		{UID: "v2", Path: "p", Title: "T", Body: "other", Visibility: "all"},
		{UID: "v3", Path: "p", Title: "T", Body: "another", Visibility: "all"},
	})
	playerAllowed := []string{"v1", "v2", "v3"}

	baseline, err := s.Query(QueryRequest{Query: `body contains "xylophone"`, Allowed: playerAllowed})
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.UIDs) != 1 || baseline.UIDs[0] != "v1" {
		t.Fatalf("baseline = %v, want [v1]", baseline.UIDs)
	}
	baselineCount := baseline.Count

	// Insert a GM-only note containing the player's rare term.
	s.UpsertNotes([]NoteRecord{
		{UID: "gm1", Path: "p", Title: "T", Body: "xylophone secret", Visibility: "gm"},
	})

	after, err := s.Query(QueryRequest{Query: `body contains "xylophone"`, Allowed: playerAllowed})
	if err != nil {
		t.Fatal(err)
	}
	if after.Count != baselineCount {
		t.Fatalf("count changed: %d -> %d (oracle)", baselineCount, after.Count)
	}
	if len(after.UIDs) != len(baseline.UIDs) {
		t.Fatalf("set size changed: %d -> %d", len(baseline.UIDs), len(after.UIDs))
	}
	for i := range baseline.UIDs {
		if after.UIDs[i] != baseline.UIDs[i] {
			t.Fatalf("order/content changed at %d: %v vs %v", i, baseline.UIDs, after.UIDs)
		}
	}
}
