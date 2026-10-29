package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func p0bPostNote(t *testing.T, srvURL, token string, body map[string]any) int {
	t.Helper()
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srvURL+"/api/v1/notes", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestP0bAnonymousCreateRejected(t *testing.T) {
	_, srv, _, _ := f3env(t)
	st := p0bPostNote(t, srv.URL, "", map[string]any{
		"type": "message", "name": "m", "visibility": "party", "campaign": "demo", "content": "hi",
	})
	if st != http.StatusUnauthorized {
		t.Fatalf("anonymous create status %d want 401", st)
	}
}

func TestP0bGMCreateAllowed(t *testing.T) {
	_, srv, gmTok, _ := f3env(t)
	st := p0bPostNote(t, srv.URL, gmTok, map[string]any{
		"type": "message", "name": "m", "visibility": "party", "campaign": "demo", "content": "hi",
	})
	if st != http.StatusOK {
		t.Fatalf("gm create status %d want 200", st)
	}
}

func TestP0bAccountMemberCreateAllowed(t *testing.T) {
	v, srv, _, _ := f3env(t)
	_ = v
	tk := tokenKeyFor(v)
	tok, err := issueToken(tk, tokenClaims{Kind: "account", ID: "alice", Campaigns: []string{"demo"}})
	if err != nil || tok == "" {
		t.Fatal("issue alice token")
	}
	st := p0bPostNote(t, srv.URL, tok, map[string]any{
		"type": "message", "name": "m", "visibility": "party", "campaign": "demo", "content": "hi",
	})
	if st != http.StatusOK {
		t.Fatalf("account member create status %d want 200", st)
	}
}

func TestP0bAccountNonMemberCreateRejected(t *testing.T) {
	v, srv, _, _ := f3env(t)
	_ = v
	tk := tokenKeyFor(v)
	tok, err := issueToken(tk, tokenClaims{Kind: "account", ID: "mallory", Campaigns: []string{"other"}})
	if err != nil || tok == "" {
		t.Fatal("issue mallory token")
	}
	st := p0bPostNote(t, srv.URL, tok, map[string]any{
		"type": "message", "name": "m", "visibility": "party", "campaign": "demo", "content": "hi",
	})
	if st != http.StatusForbidden {
		t.Fatalf("account non-member create status %d want 403", st)
	}
}

func TestP0bSeatOutsideCampaignCreateRejected(t *testing.T) {
	v, srv, gmTok, _ := f3env(t)
	_ = v
	sid, code := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid, "guest", "", "")
	seatTok, _ := f3joinByCode(t, srv, sid, code)
	if seatTok == "" {
		t.Fatal("empty seat token")
	}
	st := p0bPostNote(t, srv.URL, seatTok, map[string]any{
		"type": "message", "name": "m", "visibility": "party", "campaign": "demo", "content": "hi",
	})
	if st != http.StatusForbidden {
		t.Fatalf("seat outside campaign create status %d want 403", st)
	}
}

func TestP0bGuestCreateVisibleAllowed(t *testing.T) {
	v, srv, gmTok, _ := f3env(t)
	_ = v
	sid, code := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid, "guest", "", "")
	seatTok, _ := f3joinByCode(t, srv, sid, code)
	if seatTok == "" {
		t.Fatal("empty seat token")
	}
	// public note with no campaign is visible to a guest seat.
	st := p0bPostNote(t, srv.URL, seatTok, map[string]any{
		"type": "note", "name": "n", "visibility": "public", "content": "hi",
	})
	if st != http.StatusOK {
		t.Fatalf("guest visible create status %d want 200", st)
	}
}

func TestP0bGuestCreateInvisibleRejected(t *testing.T) {
	v, srv, gmTok, _ := f3env(t)
	_ = v
	sid, code := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid, "guest", "", "")
	seatTok, _ := f3joinByCode(t, srv, sid, code)
	if seatTok == "" {
		t.Fatal("empty seat token")
	}
	st := p0bPostNote(t, srv.URL, seatTok, map[string]any{
		"type": "message", "name": "m", "visibility": "party", "campaign": "demo", "content": "hi",
	})
	if st != http.StatusForbidden {
		t.Fatalf("guest invisible create status %d want 403", st)
	}
}

// keep imports used
var _ = os.WriteFile
var _ = filepath.Join
var _ = vault.Visible
