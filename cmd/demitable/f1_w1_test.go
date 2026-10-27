package main

import (
	"testing"
	"time"
)

func TestF1SessionCodeLimiterTimeDecay(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newSessionCodeLimiter()
	l.now = func() time.Time { return now }

	key := "s|ip"
	// 10 failures
	for i := 0; i < 10; i++ {
		if n := l.fail(key); n != i+1 {
			t.Fatalf("fail %d returned %d", i+1, n)
		}
	}
	if l.allow(key) {
		t.Fatal("expected limited after 10")
	}
	if l.count(key) != 10 {
		t.Fatalf("count=%d, want 10", l.count(key))
	}

	// advance beyond window: failures decay
	now = now.Add(sessionCodeFailureWindow + time.Second)
	if !l.allow(key) {
		t.Fatal("expected allowed after decay")
	}
	if c := l.count(key); c != 0 {
		t.Fatalf("count after decay=%d, want 0", c)
	}
	// new failures start fresh
	if n := l.fail(key); n != 1 {
		t.Fatalf("post-decay fail returned %d, want 1", n)
	}

	// partial decay: failures within window remain
	now2 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l2 := newSessionCodeLimiter()
	l2.now = func() time.Time { return now2 }
	for i := 0; i < 5; i++ {
		l2.fail(key)
	}
	// advance half window: still within
	now2 = now2.Add(sessionCodeFailureWindow / 2)
	if c := l2.count(key); c != 5 {
		t.Fatalf("half-window count=%d, want 5", c)
	}
	// advance full window from original: all decay
	now2 = now2.Add(sessionCodeFailureWindow)
	if c := l2.count(key); c != 0 {
		t.Fatalf("full-decay count=%d, want 0", c)
	}
}

func TestF1SessionCodeLimiterConstantNamed(t *testing.T) {
	if sessionCodeFailureWindow != 5*time.Minute {
		t.Fatalf("window=%v", sessionCodeFailureWindow)
	}
}
