package session

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSessionAlphabetHasNoL(t *testing.T) {
	for _, c := range sessionCodeAlphabet {
		if c == 'L' {
			t.Fatal("alphabet contains L")
		}
	}
	if len(sessionCodeAlphabet) == 0 {
		t.Fatal("empty alphabet")
	}
}

func TestClaimSeatConcurrency(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	sess := Session{ID: "s1", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()}
	if err := s.CreateSession(sess); err != nil {
		t.Fatal(err)
	}
	seat := Seat{ID: "seat1", SessionID: "s1", Kind: KindGuest, Claimed: false, CreatedAt: time.Now().UTC()}
	if err := s.AddSeat(seat); err != nil {
		t.Fatal(err)
	}

	const N = 20
	var wg sync.WaitGroup
	results := make(chan bool, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := s.ClaimSeat("s1", "")
			if err != nil {
				t.Errorf("claim err: %v", err)
				results <- false
				return
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	holders := 0
	for ok := range results {
		if ok {
			holders++
		}
	}
	if holders != 1 {
		t.Fatalf("holders = %d, want 1", holders)
	}
}

func TestCloseIdempotentStore(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.CreateSession(Session{ID: "s1", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()})
	if err := s.CloseSession("s1", "batch-1"); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetSession("s1")
	if first.ClosedAt == nil {
		t.Fatal("not closed")
	}
	closedAt := *first.ClosedAt
	batch := first.CloseBatchID
	if err := s.CloseSession("s1", "batch-2"); err != nil {
		t.Fatal(err)
	}
	second, _ := s.GetSession("s1")
	if second.CloseBatchID != batch {
		t.Fatalf("batch changed: %s vs %s", second.CloseBatchID, batch)
	}
	if !second.ClosedAt.Equal(closedAt) {
		t.Fatalf("closed_at changed")
	}
}

func TestUpdateSeatClosedEnforcement(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.CreateSession(Session{ID: "s1", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()})
	s.AddSeat(Seat{ID: "seat1", SessionID: "s1", Kind: KindGuest, Claimed: false, CreatedAt: time.Now().UTC()})
	s.CloseSession("s1", "b")
	if err := s.UpdateSeat(Seat{ID: "seat1", SessionID: "s1", Kind: KindGuest, Claimed: true}); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("UpdateSeat: got %v", err)
	}
}

func TestStoreClosedEnforcement(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	s.CreateSession(Session{ID: "s1", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()})
	s.CloseSession("s1", "b")
	if err := s.RotateCode("s1", "ZZZZZZ"); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("RotateCode: got %v, want ErrSessionClosed", err)
	}
	if err := s.AddSeat(Seat{ID: "x", SessionID: "s1", Kind: KindGuest}); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("AddSeat: got %v", err)
	}
	if err := s.SetCodeAttempts("s1", 5); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("SetCodeAttempts: got %v", err)
	}
}

func TestStoreScratchPurgeOnClose(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateSession(Session{ID: "s1", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendScratch(ScratchEntry{SessionID: "s1", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ListScratch("s1"); err != nil || len(got) != 1 {
		t.Fatalf("pre-close scratch = %d (err %v), want 1", len(got), err)
	}
	if err := s.CloseSession("s1", "batch-1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListScratch("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("post-close scratch = %d, want 0", len(got))
	}
}

func TestUpdateSessionClosedGuard(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateSession(Session{ID: "s1", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession("s1", "b"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSession(Session{ID: "s1", Code: "ZZZZZZ", Status: StatusOpen, CreatedAt: time.Now().UTC()}); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("UpdateSession on closed: got %v, want ErrSessionClosed", err)
	}
	// close still works (idempotent)
	if err := s.CloseSession("s1", "b2"); err != nil {
		t.Fatalf("close after update attempt: %v", err)
	}
	// SetCodeAttempts keeps working on an open session
	s2, _ := Open(":memory:")
	defer s2.Close()
	if err := s2.CreateSession(Session{ID: "s2", Code: "ABCDEF", Status: StatusOpen, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := s2.SetCodeAttempts("s2", 3); err != nil {
		t.Fatalf("SetCodeAttempts open: %v", err)
	}
	got, _ := s2.GetSession("s2")
	if got.CodeAttempts != 3 {
		t.Fatalf("CodeAttempts = %d, want 3", got.CodeAttempts)
	}
}
