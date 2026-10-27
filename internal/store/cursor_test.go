package store

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestCursorPagingAfterInterveningInsert(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	for _, u := range []string{"a", "b", "c", "d", "e"} {
		s.UpsertNotes([]NoteRecord{{UID: u, Path: "p", Title: "T", Body: "body"}})
	}
	res1, err := s.Query(QueryRequest{Sort: "uid", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res1.UIDs) != 2 || res1.UIDs[0] != "a" || res1.UIDs[1] != "b" {
		t.Fatalf("page1 = %v", res1.UIDs)
	}
	if res1.NextCursor == "" {
		t.Fatal("missing cursor")
	}
	// Intervening insert.
	s.UpsertNotes([]NoteRecord{{UID: "z", Path: "p", Title: "T", Body: "body"}})

	res2, err := s.Query(QueryRequest{Sort: "uid", Limit: 2, Cursor: res1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.UIDs) != 2 {
		t.Fatalf("page2 len = %d, want 2 (%v)", len(res2.UIDs), res2.UIDs)
	}
	for _, u := range res2.UIDs {
		if u == "a" || u == "b" {
			t.Fatalf("dup on page2: %v", res2.UIDs)
		}
	}
	// No duplicate of page1 items on page2.
	for _, u := range res1.UIDs {
		for _, v := range res2.UIDs {
			if u == v {
				t.Fatalf("duplicate %s across page1/page2", u)
			}
		}
	}
	// page2 should be the two items immediately after "b" in uid order.
	if len(res2.UIDs) != 2 || res2.UIDs[0] != "c" || res2.UIDs[1] != "d" {
		t.Fatalf("page2 = %v, want [c d]", res2.UIDs)
	}
}

func TestCursorForgedRejected(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{{UID: "a", Path: "p", Title: "T", Body: "body"}})
	res, _ := s.Query(QueryRequest{Sort: "uid", Limit: 1})
	_ = res
	// forged: random bytes
	forged := base64.URLEncoding.EncodeToString([]byte("not-a-real-cursor"))
	_, err := s.Query(QueryRequest{Sort: "uid", Limit: 1, Cursor: forged})
	if err == nil || !isCursorErr(err) {
		t.Fatalf("forged cursor not rejected with ErrCursor: %v", err)
	}
}

func TestCursorTamperedRejected(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{{UID: "a", Path: "p", Title: "T", Body: "body"}, {UID: "b", Path: "p", Title: "T", Body: "body"}})
	res, _ := s.Query(QueryRequest{Sort: "uid", Limit: 1})
	cur := res.NextCursor
	// tamper a byte in the middle
	raw, _ := base64.URLEncoding.DecodeString(cur)
	raw[len(raw)-1] ^= 0xff
	tampered := base64.URLEncoding.EncodeToString(raw)
	_, err := s.Query(QueryRequest{Sort: "uid", Limit: 1, Cursor: tampered})
	if err == nil || !isCursorErr(err) {
		t.Fatalf("tampered cursor not rejected with ErrCursor: %v", err)
	}
}

func TestCursorMismatchedQueryRejected(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	s.UpsertNotes([]NoteRecord{{UID: "a", Path: "p", Title: "T", Body: "body"}, {UID: "b", Path: "p", Title: "T", Body: "body"}})
	res, _ := s.Query(QueryRequest{Sort: "uid", Limit: 1})
	_, err := s.Query(QueryRequest{Sort: "uid", Limit: 1, Cursor: res.NextCursor, Desc: true})
	if err == nil || !isCursorErr(err) {
		t.Fatalf("mismatched cursor not rejected: %v", err)
	}
}

func TestCursorCountEqualsReturned(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.Migrate()
	for i := 0; i < 7; i++ {
		s.UpsertNotes([]NoteRecord{{UID: string(rune('a' + i)), Path: "p", Title: "T", Body: "body"}})
	}
	res, err := s.Query(QueryRequest{Sort: "uid", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != len(res.UIDs) {
		t.Fatalf("Count=%d but len(UIDs)=%d", res.Count, len(res.UIDs))
	}
	if res.Count != len(res.Items) {
		t.Fatalf("Count=%d but len(Items)=%d", res.Count, len(res.Items))
	}
	if res.Count != 3 {
		t.Fatalf("Count=%d, want page size 3 (no total-matched)", res.Count)
	}
}

func isCursorErr(err error) bool {
	return err != nil && errors.Is(err, ErrCursor)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
