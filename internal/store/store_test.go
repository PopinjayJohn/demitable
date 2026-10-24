package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestStorePragmas(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var bt int
	s.DB().QueryRow("PRAGMA busy_timeout").Scan(&bt)
	if bt != 5000 {
		t.Errorf("busy_timeout=%d", bt)
	}
}

func TestConcurrentInserts(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Migrate()
	var wg sync.WaitGroup
	errs := make(chan error, 400)
	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.UpsertNote(fmt.Sprintf("uid%d", i), "p.md", "title", nil, nil, nil, "body"); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	var n int
	for e := range errs {
		n++
		t.Logf("err: %v", e)
	}
	if n != 0 {
		t.Fatalf("%d of 400 failed", n)
	}
}
