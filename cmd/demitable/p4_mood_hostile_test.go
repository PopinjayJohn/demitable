package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func p4SetCues(t *testing.T, srv *httptest.Server, gmTok, sid string) {
	t.Helper()
	body := `[{"mood":"tense","src":"/asset/tense.mp3","title":"GM secret title"}]`
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/cues", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("set cues: %d %s", resp.StatusCode, string(b))
	}
}

// TestP4CuesTitleProjectionHostile pins ADR-0048/0057: non-GM cue GETs must
// not contain a "title" key or the GM-chosen title value in the raw bytes.
func TestP4CuesTitleProjectionHostile(t *testing.T) {
	dir, v, srv, gmTok := f1Env(t)
	_ = dir
	sid, code := f1CreateSession(t, srv, gmTok)
	p4SetCues(t, srv, gmTok, sid)

	tk := tokenKeyFor(v)
	accountTok, err := issueToken(tk, tokenClaims{Kind: "account", ID: "p4player", Campaigns: []string{"C"}})
	if err != nil {
		t.Fatal(err)
	}
	otherSid, otherCode := f1CreateSession(t, srv, gmTok)
	otherSeatTok := p1GuestSeatToken(t, srv, otherSid, otherCode)
	_ = code

	cases := []struct {
		name      string
		tok       string
		wantTitle bool
	}{
		{"gm", gmTok, true},
		{"anon", "", false},
		{"player-account", accountTok, false},
		{"other-session-seat", otherSeatTok, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/cues", nil)
			if tc.tok != "" {
				req.Header.Set("Authorization", "Bearer "+tc.tok)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d want 200: %s", resp.StatusCode, string(body))
			}
			hasTitle := strings.Contains(string(body), `"title"`)
			hasSecret := strings.Contains(string(body), "GM secret title")
			if tc.wantTitle {
				if !hasTitle {
					t.Fatalf("GM response missing title key: %s", string(body))
				}
			} else {
				if hasTitle {
					t.Fatalf("non-GM response leaked title key: %s", string(body))
				}
				if hasSecret {
					t.Fatalf("non-GM response leaked title value: %s", string(body))
				}
			}
		})
	}
}

// TestP4CueSelectHostile pins the server-side seat check on cue select.
func TestP4CueSelectHostile(t *testing.T) {
	dir, v, srv, gmTok := f1Env(t)
	_ = dir
	sid, code := f1CreateSession(t, srv, gmTok)
	p4SetCues(t, srv, gmTok, sid)

	tk := tokenKeyFor(v)
	forgedSeat, _ := issueToken(tk, tokenClaims{Kind: "seat", ID: "forged-seat", Bound: "x", Session: sid, Campaigns: []string{"C"}})
	guestTok := p1GuestSeatToken(t, srv, sid, code)
	displayTok, _ := issueToken(tk, tokenClaims{Kind: "display", Campaigns: []string{"C"}})
	playerTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p4player2", Campaigns: []string{"C"}})

	cases := []struct {
		name string
		tok  string
	}{
		{"forged-seat", forgedSeat},
		{"guest", guestTok},
		{"display", displayTok},
		{"player", playerTok},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/cues/select", bytes.NewBufferString(`{"mood":"tense"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.tok)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("token=%s status=%d want 403: %s", tc.name, resp.StatusCode, string(body))
			}
		})
	}
}

// TestP4MoodOtherSessionTokenHostile pins cross-session seat rejection on mood.
func TestP4MoodOtherSessionTokenHostile(t *testing.T) {
	_, v, srv, gmTok := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmTok)
	otherSid, otherCode := f1CreateSession(t, srv, gmTok)
	otherSeatTok := p1GuestSeatToken(t, srv, otherSid, otherCode)
	_ = v

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			var resp *http.Response
			var err error
			url := srv.URL + "/api/v1/sessions/" + sid + "/mood"
			if method == http.MethodGet {
				req, _ := http.NewRequest(method, url, nil)
				req.Header.Set("Authorization", "Bearer "+otherSeatTok)
				resp, err = http.DefaultClient.Do(req)
			} else {
				req, _ := http.NewRequest(method, url, bytes.NewBufferString(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+otherSeatTok)
				resp, err = http.DefaultClient.Do(req)
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("method=%s status=%d want 401: %s", method, resp.StatusCode, string(body))
			}
		})
	}
}

// TestP4CuesUnknownSession pins 404 not_found for unknown sessions.
func TestP4CuesUnknownSession(t *testing.T) {
	_, _, srv, gmTok := f1Env(t)
	for _, tc := range []struct {
		name string
		tok  string
	}{
		{"anon", ""},
		{"gm", gmTok},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/ses-does-not-exist/cues", nil)
			if tc.tok != "" {
				req.Header.Set("Authorization", "Bearer "+tc.tok)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status=%d want 404: %s", resp.StatusCode, string(body))
			}
			if !strings.Contains(string(body), "not_found") {
				t.Fatalf("body missing not_found: %s", string(body))
			}
		})
	}
}
