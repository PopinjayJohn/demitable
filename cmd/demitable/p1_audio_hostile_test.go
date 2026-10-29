package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// p1AudioEnv wraps f1Env with an isolated audio data dir.
func p1AudioEnv(t *testing.T) (string, *vault.Vault, *httptest.Server, string) {
	t.Helper()
	dir, v, srv, gmTok := f1Env(t)
	t.Setenv("DEMITABLE_DATA_DIR", t.TempDir())
	return dir, v, srv, gmTok
}

func p1GuestSeatToken(t *testing.T, srv *httptest.Server, sid, code string) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("guest join %d: %+v", resp.StatusCode, j)
	}
	tok, _ := j["token"].(string)
	if tok == "" {
		t.Fatal("missing guest seat token")
	}
	return tok
}

func p1GMSeatToken(t *testing.T, srv *httptest.Server, gmTok, sid string) string {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/join", bytes.NewBufferString(`{}`))
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
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm join %d: %+v", resp.StatusCode, j)
	}
	tok, _ := j["token"].(string)
	if tok == "" {
		t.Fatal("missing gm seat token")
	}
	return tok
}

// TestP1AudioPostHostile: POST auth gate is load-bearing.
func TestP1AudioPostHostile(t *testing.T) {
	dir, v, srv, gmTok := p1AudioEnv(t)
	_ = dir
	_ = v
	targetSid, _ := f1CreateSession(t, srv, gmTok)
	otherSid, otherCode := f1CreateSession(t, srv, gmTok)
	otherGuestTok := p1GuestSeatToken(t, srv, otherSid, otherCode)
	otherGMTok := p1GMSeatToken(t, srv, gmTok, otherSid)

	tk := tokenKeyFor(v)
	accountTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"C"}})

	tokens := []struct {
		name string
		tok  string
	}{
		{"account", accountTok},
		{"anon", ""},
		{"other-session-guest-seat", otherGuestTok},
		{"other-session-gm-seat", otherGMTok},
	}
	targets := []string{targetSid, "ses-guessed-does-not-exist"}
	for _, sid := range targets {
		for _, tc := range tokens {
			data := []byte("hostile post payload")
			req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader(data))
			if tc.tok != "" {
				req.Header.Set("Authorization", "Bearer "+tc.tok)
			}
			req.Header.Set("Content-Type", "application/octet-stream")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("POST sid=%s token=%s: status=%d want 404", sid, tc.name, resp.StatusCode)
			}
			if len(body) != 0 {
				t.Fatalf("POST sid=%s token=%s: body bytes=%d want 0", sid, tc.name, len(body))
			}
		}
	}
}

// TestP1AudioDeleteHostile: DELETE auth gate is load-bearing.
func TestP1AudioDeleteHostile(t *testing.T) {
	dir, v, srv, gmTok := p1AudioEnv(t)
	_ = dir
	_ = v
	targetSid, _ := f1CreateSession(t, srv, gmTok)
	// pre-upload so a non-gated delete would actually remove something
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+targetSid+"/audio", bytes.NewReader([]byte("x")))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	r, _ := http.DefaultClient.Do(req)
	if r != nil {
		r.Body.Close()
	}
	otherSid, otherCode := f1CreateSession(t, srv, gmTok)
	otherGuestTok := p1GuestSeatToken(t, srv, otherSid, otherCode)
	otherGMTok := p1GMSeatToken(t, srv, gmTok, otherSid)

	tk := tokenKeyFor(v)
	accountTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"C"}})

	tokens := []struct {
		name string
		tok  string
	}{
		{"account", accountTok},
		{"anon", ""},
		{"other-session-guest-seat", otherGuestTok},
		{"other-session-gm-seat", otherGMTok},
	}
	targets := []string{targetSid, "ses-guessed-does-not-exist"}
	for _, sid := range targets {
		for _, tc := range tokens {
			dreq, _ := http.NewRequest("DELETE", srv.URL+"/api/v1/sessions/"+sid+"/audio", nil)
			if tc.tok != "" {
				dreq.Header.Set("Authorization", "Bearer "+tc.tok)
			}
			resp, err := http.DefaultClient.Do(dreq)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("DELETE sid=%s token=%s: status=%d want 404", sid, tc.name, resp.StatusCode)
			}
			if len(body) != 0 {
				t.Fatalf("DELETE sid=%s token=%s: body bytes=%d want 0", sid, tc.name, len(body))
			}
		}
	}
}

// TestP1AudioGetOtherSessionSeat: GET auth gate for other-session seat tokens.
func TestP1AudioGetOtherSessionSeat(t *testing.T) {
	dir, v, srv, gmTok := p1AudioEnv(t)
	_ = dir
	_ = v
	targetSid, _ := f1CreateSession(t, srv, gmTok)
	// pre-upload audio so a non-gated read would return bytes
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+targetSid+"/audio", bytes.NewReader([]byte("secret audio bytes")))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	r, _ := http.DefaultClient.Do(req)
	if r != nil {
		r.Body.Close()
	}
	otherSid, otherCode := f1CreateSession(t, srv, gmTok)
	otherGuestTok := p1GuestSeatToken(t, srv, otherSid, otherCode)
	otherGMTok := p1GMSeatToken(t, srv, gmTok, otherSid)

	for _, tc := range []struct {
		name string
		tok  string
	}{
		{"other-session-guest-seat", otherGuestTok},
		{"other-session-gm-seat", otherGMTok},
	} {
		greq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+targetSid+"/audio", nil)
		greq.Header.Set("Authorization", "Bearer "+tc.tok)
		resp, err := http.DefaultClient.Do(greq)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET token=%s: status=%d want 404", tc.name, resp.StatusCode)
		}
		if len(body) != 0 {
			t.Fatalf("GET token=%s: body bytes=%d want 0", tc.name, len(body))
		}
	}

	// guessed session id -> 404 + 0 bytes
	for _, tc := range []struct {
		name string
		tok  string
	}{
		{"other-session-guest-seat", otherGuestTok},
		{"other-session-gm-seat", otherGMTok},
	} {
		greq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/ses-guessed-does-not-exist/audio", nil)
		greq.Header.Set("Authorization", "Bearer "+tc.tok)
		resp, err := http.DefaultClient.Do(greq)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET guessed token=%s: status=%d want 404", tc.name, resp.StatusCode)
		}
		if len(body) != 0 {
			t.Fatalf("GET guessed token=%s: body bytes=%d want 0", tc.name, len(body))
		}
	}
}

// TestP1AudioWSNoLeak: a player seat hears nothing during a GM upload.
func TestP1AudioWSNoLeak(t *testing.T) {
	dir, v, srv, gmTok := p1AudioEnv(t)
	_ = dir
	_ = v
	sid, code := f1CreateSession(t, srv, gmTok)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	playerTok, _ := j["token"].(string)
	if playerTok == "" {
		t.Fatal("missing player seat token")
	}

	pc := dialWSN7(t, srv, playerTok)
	defer pc.Close()
	readFrameN7(pc) // hello
	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc) // hello

	go func() {
		rq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader([]byte("ws leak payload")))
		rq.Header.Set("Authorization", "Bearer "+gmTok)
		rq.Header.Set("Content-Type", "application/octet-stream")
		r, _ := http.DefaultClient.Do(rq)
		if r != nil {
			r.Body.Close()
		}
	}()

	// GM connection must receive the broadcast (positive control).
	gc.SetReadDeadline(time.Now().Add(5 * time.Second))
	gmFrame, err := readFrameN7(gc)
	if err != nil {
		t.Fatalf("gm received no frame: %v", err)
	}
	if !strings.Contains(string(gmFrame), sid) && !strings.Contains(string(gmFrame), "audio") {
		t.Fatalf("gm frame unexpected: %s", string(gmFrame))
	}

	// Player connection must receive nothing beyond hello.
	pc.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	pFrame, err := readFrameN7(pc)
	if err == nil {
		t.Fatalf("player received unexpected frame: %s", string(pFrame))
	}
	_ = time.Second
}

// TestP1CaptureLanProperty: capture.ts uses relative URLs only (LAN property).
func TestP1CaptureLanProperty(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "capture.ts"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"http://", "https://", "://"} {
		if strings.Contains(s, bad) {
			t.Fatalf("capture.ts contains remote/absolute URL marker %q", bad)
		}
	}
	if !strings.Contains(s, "apiFetch") {
		t.Fatal("capture.ts does not use apiFetch (relative, same-origin)")
	}
}
