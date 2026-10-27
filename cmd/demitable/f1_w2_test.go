package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestF1UnknownSeatTokenResolvesAnonymous(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	_ = v
	sid, _ := f1CreateSession(t, srv, gmToken)
	tk := tokenKeyFor(v)
	// mint a valid-signature token for a NONEXISTENT seat in an OPEN session
	tok, err := issueToken(tk, tokenClaims{
		Kind:    "seat",
		ID:      "seat-does-not-exist",
		Bound:   "charX",
		Session: sid,
	})
	if err != nil {
		t.Fatal(err)
	}
	// session is open
	st := session.StoreFor(tk)
	if st == nil || !st.IsOpen(sid) {
		t.Fatal("session not open")
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	// buys nothing
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown seat token status %d, want 401/404", resp.StatusCode)
	}
	// direct resolver check
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	p := principalFromRequest(r, tk)
	if p.IsSeat() {
		t.Fatal("unknown seat resolved as seat")
	}
	if p.Kind != "anonymous" {
		t.Fatalf("kind=%s, want anonymous", p.Kind)
	}
	_ = bytes.NewBuffer
	_ = json.NewDecoder
	_ = fmt.Sprint
	_ = httptest.NewServer
	_ = vault.Anonymous
}
