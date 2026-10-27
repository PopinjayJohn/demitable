package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func hiddenCorpusCount(t *testing.T, srvURL, token string) int {
	t.Helper()
	uids, count := oracleQueryUIDs(t, srvURL, token, oracleRare)
	_ = uids
	return count
}

func TestF1ReadPathWrongSessionSeatToken(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	buildAdversarialCorpus(t, dir, v)
	// add a visible rare note? not needed
	sid, _ := f1CreateSession(t, srv, gmToken)
	// mint token pointing at a WRONG session id
	tk := tokenKeyFor(v)
	tok, _ := issueToken(tk, tokenClaims{Kind: "seat", ID: "seat-x", Bound: "charX", Session: "no-such-session"})
	// ensure open session exists for sid but token session differs
	_ = sid
	_ = gmToken
	// out-of-session token resolves anonymous: hidden corpus invisible
	if c := hiddenCorpusCount(t, srv.URL, tok); c != 0 {
		t.Fatalf("wrong-session seat token saw %d rare notes, want 0", c)
	}
	// direct resolver
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	p := principalFromRequest(r, tk)
	if p.IsSeat() {
		t.Fatal("wrong-session seat resolved as seat")
	}
}

func TestF1ReadPathEndedSessionSeatToken(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	buildAdversarialCorpus(t, dir, v)
	sid, code := f1CreateSession(t, srv, gmToken)
	// join to get valid seat token
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	tok := j["token"].(string)
	// close session
	cr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	_ = err
	cr.Header.Set("Authorization", "Bearer "+gmToken)
	cresp, _ := http.DefaultClient.Do(cr)
	cresp.Body.Close()
	// token now out-of-session (ended): hidden corpus invisible
	if c := hiddenCorpusCount(t, srv.URL, tok); c != 0 {
		t.Fatalf("ended-session seat token saw %d rare notes, want 0", c)
	}
}

func TestF1ReadPathNonSessionSeatToken(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	buildAdversarialCorpus(t, dir, v)
	// mint a seat token with no Session field (non-session)
	tk := tokenKeyFor(v)
	tok, _ := issueToken(tk, tokenClaims{Kind: "seat", ID: "seat-y", Bound: "charY"})
	_ = gmToken
	if c := hiddenCorpusCount(t, srv.URL, tok); c != 0 {
		t.Fatalf("non-session seat token saw %d rare notes, want 0", c)
	}
	// also assert a GM-only hidden note uid absent
	uids, _ := oracleQueryUIDs(t, srv.URL, tok, oracleRare)
	for _, u := range uids {
		if u == "hidden_gm" {
			t.Fatal("hidden_gm leaked via non-session seat token")
		}
	}
}
