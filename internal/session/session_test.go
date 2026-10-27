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
