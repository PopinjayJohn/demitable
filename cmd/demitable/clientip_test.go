package main

import (
	"net/http/httptest"
	"testing"
	"time"
)

// The rate limiter keys on the client address. It must key on the
// *address*, not on the socket's ephemeral port: keying on the port
// gives every request a fresh key, and the limiter never engages.
func TestAuthLimiterEngages(t *testing.T) {
	l := newAuthLimiter()
	l.fail("10.0.0.1")
	if l.allow("10.0.0.1") {
		t.Fatal("the limiter allowed a source immediately after a failure")
	}
	if !l.allow("10.0.0.2") {
		t.Fatal("the limiter blocked an unrelated source")
	}
	l.success("10.0.0.1")
	if !l.allow("10.0.0.1") {
		t.Fatal("a success did not clear the limiter")
	}
}

// The backoff must grow, and it must be bounded.
func TestAuthLimiterBackoffGrowsAndIsBounded(t *testing.T) {
	l := newAuthLimiter()
	l.fail("10.0.0.1")
	first := l.state["10.0.0.1"].next.Sub(time.Now())
	l.fail("10.0.0.1")
	second := l.state["10.0.0.1"].next.Sub(time.Now())
	if second <= first {
		t.Fatalf("backoff did not grow: %v then %v", first, second)
	}
	for i := 0; i < 40; i++ {
		l.fail("10.0.0.1")
	}
	d := l.state["10.0.0.1"].next.Sub(time.Now())
	if d > 6*time.Minute {
		t.Fatalf("backoff is unbounded: %v", d)
	}
}

// A forwarded header is only believed when the request came from the
// configured proxy (ADR-0031). Otherwise it is attacker-controlled, and a
// limiter keyed on it is a limiter an attacker can reset.
func TestForwardedHeaderOnlyFromTrustedProxy(t *testing.T) {
	defer setTrustedProxy("")
	setTrustedProxy("")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("an untrusted forwarded header was believed: got %q", got)
	}

	// Now trust the peer.
	setTrustedProxy("203.0.113.9")
	defer setTrustedProxy("")
	if got := clientIP(r); got != "1.2.3.4" {
		t.Fatalf("a trusted proxy's forwarded header was ignored: got %q", got)
	}

	// A different peer is still not trusted.
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "198.51.100.4:5555"
	r2.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r2); got != "198.51.100.4" {
		t.Fatalf("a forwarded header from an untrusted peer was believed: got %q", got)
	}
}

// A CIDR must work as well as a bare IP.
func TestTrustedProxyAcceptsCIDR(t *testing.T) {
	defer setTrustedProxy("")
	setTrustedProxy("203.0.113.0/24")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "1.2.3.4" {
		t.Fatalf("a CIDR proxy was not honoured: got %q", got)
	}
}
