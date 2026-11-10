package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// ADR-0067: only the GM or the note's owner may change visibility or
// ownership, on both PATCH and PUT. Setup keeps the vault handle so
// account/seat tokens can be minted directly.
func fieldAuthSetup(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := runInit([]string{dir}); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(muxWithVault(v))
	t.Cleanup(srv.Close)
	gmSecret, _, err := ensureVaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(srv.URL+"/api/v1/auth/gm", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"secret":%q}`, gmSecret)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	tk := tokenKeyFor(v)
	return srv, body["token"].(string), tk
}

func fieldAuthCreateSession(t *testing.T, srv *httptest.Server, gmToken string) (string, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions", bytes.NewBufferString(`{"name":"s","campaign":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var s map[string]any
	json.NewDecoder(resp.Body).Decode(&s)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create session %d: %+v", resp.StatusCode, s)
	}
	return s["id"].(string), s["code"].(string)
}

// fieldAuthGuestSeat pre-assigns a guest seat bound to character (a note
// uid or a player id) and joins it with the session code, returning the
// seat token.
func fieldAuthGuestSeat(t *testing.T, srv *httptest.Server, gmToken, sid, code, character string) string {
	t.Helper()
	bb, _ := json.Marshal(map[string]any{"kind": "guest", "character": character})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/seats", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var o map[string]any
		json.NewDecoder(resp.Body).Decode(&o)
		t.Fatalf("add seat %d: %+v", resp.StatusCode, o)
	}
	jr, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer jr.Body.Close()
	var j map[string]any
	json.NewDecoder(jr.Body).Decode(&j)
	if jr.StatusCode != http.StatusOK {
		t.Fatalf("join %d: %+v", jr.StatusCode, j)
	}
	tok, _ := j["token"].(string)
	if tok == "" {
		t.Fatalf("no seat token: %+v", j)
	}
	return tok
}

func fieldAuthToken(t *testing.T, tk string, kind, id, bound string) string {
	t.Helper()
	tok, err := issueToken(tk, tokenClaims{Kind: kind, ID: id, Bound: bound, Campaigns: []string{"demo"}})
	if err != nil || tok == "" {
		t.Fatalf("issue %s token: %v", kind, err)
	}
	return tok
}

func fieldAuthPatch(t *testing.T, srv *httptest.Server, tok, uid, payload string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/notes/"+uid, bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestVisibilityChangeRequiresOwnerOrGM(t *testing.T) {
	srv, gmTok, tk := fieldAuthSetup(t)
	alice := fieldAuthToken(t, tk, "account", "alice", "")
	bob := fieldAuthToken(t, tk, "account", "bob", "")

	// GM creates a party-visible character owned by alice.
	create := `{"type":"character","name":"FieldChar","visibility":"party","player":"alice","campaign":"demo","content":"body"}`
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewBufferString(create))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	resp.Body.Close()
	uid, _ := c["uid"].(string)
	if uid == "" {
		t.Fatalf("no uid for created note: %+v", c)
	}

	// Stranger cannot re-audience, in any envelope shape.
	if st, _ := fieldAuthPatch(t, srv, bob, uid, `{"fields":{"visibility":"gm"}}`); st != http.StatusForbidden {
		t.Fatalf("bob PATCH visibility: got %d want 403", st)
	}
	if st, _ := fieldAuthPatch(t, srv, bob, uid, `{"visibility":"public"}`); st != http.StatusForbidden {
		t.Fatalf("bob top-level PATCH visibility: got %d want 403", st)
	}
	// Stranger cannot take ownership (the bypass that would defeat the rule).
	if st, _ := fieldAuthPatch(t, srv, bob, uid, `{"fields":{"player":"bob"}}`); st != http.StatusForbidden {
		t.Fatalf("bob PATCH player: got %d want 403", st)
	}
	// Stranger can still edit ordinary fields.
	if st, _ := fieldAuthPatch(t, srv, bob, uid, `{"fields":{"name":"FieldChar2"}}`); st != http.StatusOK {
		t.Fatalf("bob PATCH name: got %d want 200", st)
	}
	// Anonymous learns nothing.
	if st, _ := fieldAuthPatch(t, srv, "", uid, `{"fields":{"visibility":"gm"}}`); st != http.StatusNotFound {
		t.Fatalf("anon PATCH visibility: got %d want 404", st)
	}

	// Owner can narrow.
	if st, _ := fieldAuthPatch(t, srv, alice, uid, `{"fields":{"visibility":"owner"}}`); st != http.StatusOK {
		t.Fatalf("alice PATCH visibility: got %d want 200", st)
	}
	// Owner can delegate ownership; the new owner inherits the rule.
	if st, _ := fieldAuthPatch(t, srv, alice, uid, `{"fields":{"player":"bob"}}`); st != http.StatusOK {
		t.Fatalf("alice PATCH player: got %d want 200", st)
	}
	if st, _ := fieldAuthPatch(t, srv, bob, uid, `{"fields":{"visibility":"party"}}`); st != http.StatusOK {
		t.Fatalf("bob (new owner) PATCH visibility: got %d want 200", st)
	}
	if st, _ := fieldAuthPatch(t, srv, alice, uid, `{"fields":{"visibility":"public"}}`); st != http.StatusForbidden {
		t.Fatalf("alice (former owner) PATCH visibility: got %d want 403", st)
	}
	// GM can always re-audience.
	if st, _ := fieldAuthPatch(t, srv, gmTok, uid, `{"fields":{"visibility":"gm"}}`); st != http.StatusOK {
		t.Fatalf("gm PATCH visibility: got %d want 200", st)
	}
	if st, _ := fieldAuthPatch(t, srv, gmTok, uid, `{"fields":{"visibility":"party","player":"alice"}}`); st != http.StatusOK {
		t.Fatalf("gm restore: got %d want 200", st)
	}

	// A seat bound to the owned character by player id is owner-equivalent.
	// A guest seat bound by note uid to someone else's character is not.
	sid, scode := fieldAuthCreateSession(t, srv, gmTok)
	seatPlayerTok := fieldAuthGuestSeat(t, srv, gmTok, sid, scode, "alice")
	if st, _ := fieldAuthPatch(t, srv, seatPlayerTok, uid, `{"fields":{"visibility":"owner"}}`); st != http.StatusOK {
		t.Fatalf("player-bound seat PATCH visibility: got %d want 200", st)
	}
	seatUidTok := fieldAuthGuestSeat(t, srv, gmTok, sid, scode, uid)
	if st, _ := fieldAuthPatch(t, srv, seatUidTok, uid, `{"fields":{"visibility":"public"}}`); st != http.StatusForbidden {
		t.Fatalf("uid-bound guest seat PATCH visibility: got %d want 403", st)
	}
}

func TestPutVisibilityChangeRequiresOwnerOrGM(t *testing.T) {
	srv, gmTok, tk := fieldAuthSetup(t)
	alice := fieldAuthToken(t, tk, "account", "alice", "")
	bob := fieldAuthToken(t, tk, "account", "bob", "")

	create := `{"type":"character","name":"PutChar","visibility":"party","player":"alice","campaign":"demo","content":"body"}`
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewBufferString(create))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmTok)
	resp, _ := http.DefaultClient.Do(req)
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	resp.Body.Close()
	uid, _ := c["uid"].(string)
	if uid == "" {
		t.Fatalf("no uid: %+v", c)
	}

	getRaw := func(tok string) string {
		t.Helper()
		r, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/"+uid, nil)
		r.Header.Set("Authorization", "Bearer "+tok)
		rp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer rp.Body.Close()
		var g map[string]any
		json.NewDecoder(rp.Body).Decode(&g)
		raw, _ := g["content"].(string)
		return raw
	}
	putRaw := func(tok, raw string) int {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"content": raw})
		r, _ := http.NewRequest("PUT", srv.URL+"/api/v1/notes/"+uid, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+tok)
		rp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer rp.Body.Close()
		return rp.StatusCode
	}

	raw := getRaw(gmTok)
	if !strings.Contains(raw, "visibility: party") {
		t.Fatalf("expected visibility line in raw:\n%s", raw)
	}
	smuggled := strings.Replace(raw, "visibility: party", "visibility: gm", 1)

	// Stranger cannot smuggle a visibility change through a full save.
	if st := putRaw(bob, smuggled); st != http.StatusForbidden {
		t.Fatalf("bob PUT smuggled visibility: got %d want 403", st)
	}
	// Stranger can still save body edits that leave audience untouched.
	if st := putRaw(bob, raw); st != http.StatusOK {
		t.Fatalf("bob PUT unchanged audience: got %d want 200", st)
	}
	// Owner can re-audience through PUT.
	owned := strings.Replace(raw, "visibility: party", "visibility: owner", 1)
	if st := putRaw(alice, owned); st != http.StatusOK {
		t.Fatalf("alice PUT visibility: got %d want 200", st)
	}
	// GM can always save.
	if st := putRaw(gmTok, smuggled); st != http.StatusOK {
		t.Fatalf("gm PUT visibility: got %d want 200", st)
	}
}
