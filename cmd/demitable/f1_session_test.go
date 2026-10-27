package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func f1Env(t *testing.T) (string, *vault.Vault, *httptest.Server, string) {
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
	resp, err := http.Post(srv.URL+"/api/v1/auth/gm", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"secret":%q}`, gmSecret)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm auth %d: %+v", resp.StatusCode, body)
	}
	return dir, v, srv, body["token"].(string)
}

func f1CreateSession(t *testing.T, srv *httptest.Server, gmToken string) (string, string) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions", bytes.NewBufferString(`{"name":"s","campaign":"C"}`))
	if err != nil {
		t.Fatal(err)
	}
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

func f1AddNote(t *testing.T, dir string, v *vault.Vault, uid, player, vis string) {
	t.Helper()
	content := fmt.Sprintf("---\ntype: character\nuid: %s\nname: %s\ncampaign: \"[[C]]\"\nplayer: %s\nvisibility: %s\n---\nbody\n", uid, uid, player, vis)
	if err := os.WriteFile(filepath.Join(dir, uid+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
}

func f1MintAccount(t *testing.T, srv *httptest.Server, gmToken string) (string, string, string) {
	t.Helper()
	// mint player join code
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"player1"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var pc map[string]any
	json.NewDecoder(resp.Body).Decode(&pc)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mint player %d: %+v", resp.StatusCode, pc)
	}
	code := pc["join_code"].(string)
	// join
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Body.Close()
	var j map[string]any
	json.NewDecoder(r1.Body).Decode(&j)
	player := j["player"].(string)
	joinToken := j["token"].(string)
	// set passphrase
	body := map[string]any{"passphrase": "pass123"}
	bb, _ := json.Marshal(body)
	req2, err := http.NewRequest("POST", srv.URL+"/api/v1/auth/set-passphrase", bytes.NewReader(bb))
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+joinToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	// login
	lr, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "pass123")))
	if err != nil {
		t.Fatal(err)
	}
	defer lr.Body.Close()
	var lb map[string]any
	json.NewDecoder(lr.Body).Decode(&lb)
	return player, lb["token"].(string), code
}

func f1AddSeat(t *testing.T, srv *httptest.Server, gmToken, sid, kind, char, account string) map[string]any {
	t.Helper()
	body := map[string]any{"kind": kind, "character": char, "account": account}
	bb, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/seats", bytes.NewReader(bb))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add seat %d: %+v", resp.StatusCode, out)
	}
	return out["seat"].(map[string]any)
}

func TestF1JoinByCodeGuestSeat(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	_ = v
	f1AddNote(t, dir, v, "char1", "", "gm")
	sid, code := f1CreateSession(t, srv, gmToken)
	f1AddSeat(t, srv, gmToken, sid, "guest", "char1", "")
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join %d: %+v", resp.StatusCode, j)
	}
	seat := j["seat"].(map[string]any)
	if seat["kind"] != "guest" {
		t.Fatalf("kind=%v", seat["kind"])
	}
	if seat["character"] != "char1" {
		t.Fatalf("char=%v", seat["character"])
	}
	if j["token"] == nil || j["token"] == "" {
		t.Fatal("missing token")
	}
}

func TestF1JoinByPassphraseAccountAndSpectator(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	player, accountToken, _ := f1MintAccount(t, srv, gmToken)
	_ = accountToken
	f1AddNote(t, dir, v, "charA", player, "party")
	sid, _ := f1CreateSession(t, srv, gmToken)
	// account seat with own character
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123","character":"charA"}`, player)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("account join %d: %+v", resp.StatusCode, j)
	}
	seat := j["seat"].(map[string]any)
	if seat["kind"] != "account" {
		t.Fatalf("kind=%v", seat["kind"])
	}
	if seat["character"] != "charA" {
		t.Fatalf("char=%v", seat["character"])
	}

	// spectator (no character)
	resp2, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123"}`, player)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var j2 map[string]any
	json.NewDecoder(resp2.Body).Decode(&j2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("spectator join %d: %+v", resp2.StatusCode, j2)
	}
	seat2 := j2["seat"].(map[string]any)
	if seat2["kind"] != "spectator" {
		t.Fatalf("kind=%v", seat2["kind"])
	}
}

func TestF1GMJoinSeatsGMKind(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	// POST join with GM bearer
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/join", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
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
	seat := j["seat"].(map[string]any)
	if seat["kind"] != "gm" {
		t.Fatalf("kind=%v", seat["kind"])
	}
	if seat["character"] != "" && seat["character"] != nil {
		t.Fatalf("char=%v", seat["character"])
	}
}

func TestF1UnplayableCharactersRejected(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	player, _, _ := f1MintAccount(t, srv, gmToken)
	otherPlayer := "p_other"
	// unknown uid
	f1AddNote(t, dir, v, "charUnknown", "", "party") // will overwrite? use unique
	// Actually remove by not creating; use missing uid
	sid, _ := f1CreateSession(t, srv, gmToken)

	cases := []struct {
		name string
		uid  string
		mk   func()
	}{
		{"unknown", "does-not-exist", nil},
	}
	_ = cases

	// gm-visibility note
	f1AddNote(t, dir, v, "charGM", "", "gm")
	// another player's private note
	f1AddNote(t, dir, v, "charPriv", otherPlayer, "private")
	// unassigned NPC (no player)
	f1AddNote(t, dir, v, "charNPC", "", "party")
	// another player's owner-scoped character
	f1AddNote(t, dir, v, "charOwner", otherPlayer, "owner")

	for _, tc := range []struct {
		uid string
	}{
		{"does-not-exist"},
		{"charGM"},
		{"charPriv"},
		{"charNPC"},
		{"charOwner"},
	} {
		resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
			bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123","character":%q}`, player, tc.uid)))
		if err != nil {
			t.Fatal(err)
		}
		var j map[string]any
		json.NewDecoder(resp.Body).Decode(&j)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("expected reject for %s, got 200: %+v", tc.uid, j)
		}
		// named error
		errObj, _ := j["error"].(map[string]any)
		code, _ := errObj["code"].(string)
		if code != "invalid_character" {
			// maybe top-level? check string
			b, _ := json.Marshal(j)
			if !strings.Contains(string(b), "invalid_character") {
				t.Fatalf("uid %s: expected invalid_character, got %d %+v", tc.uid, resp.StatusCode, j)
			}
		}
	}
}

func TestF1RateLimitExactBoundary(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, code := f1CreateSession(t, srv, gmToken)
	wrong := "ZZZZZZ"
	// attempts 1-10 wrong -> 401
	for i := 1; i <= 10; i++ {
		resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
			bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, wrong)))
		if err != nil {
			t.Fatal(err)
		}
		var j map[string]any
		json.NewDecoder(resp.Body).Decode(&j)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d, want 401", i, resp.StatusCode)
		}
		errObj, _ := j["error"].(map[string]any)
		if errObj["code"] != "invalid_code" {
			t.Fatalf("attempt %d: code %v", i, errObj["code"])
		}
		// count exposed
		attempts := 0
		if v, ok := j["attempts"].(float64); ok {
			attempts = int(v)
		}
		if attempts != i {
			t.Fatalf("attempt %d: attempts=%d, want %d", i, attempts, i)
		}
	}
	// 11th -> 429
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, wrong)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("11th status %d, want 429", resp.StatusCode)
	}
	// while limited, correct code still limited
	resp2, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("correct while limited status %d, want 429", resp2.StatusCode)
	}

	// success resets count: use new session+ip? same ip limited globally? now per session+ip, so new session allowed.
	sid2, code2 := f1CreateSession(t, srv, gmToken)
	// wrong 3 times
	for i := 0; i < 3; i++ {
		r, err := http.Post(srv.URL+"/api/v1/sessions/"+sid2+"/join", "application/json", bytes.NewBufferString(`{"code":"ZZZZZZ"}`))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
	// correct code succeeds
	r, err := http.Post(srv.URL+"/api/v1/sessions/"+sid2+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code2)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("success join status %d", r.StatusCode)
	}
	// after success, wrong attempts reset: counter should be 0; one wrong -> attempts 1
	r2, err := http.Post(srv.URL+"/api/v1/sessions/"+sid2+"/join", "application/json", bytes.NewBufferString(`{"code":"ZZZZZZ"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	var j2 map[string]any
	json.NewDecoder(r2.Body).Decode(&j2)
	attempts := 0
	if v, ok := j2["attempts"].(float64); ok {
		attempts = int(v)
	}
	if attempts != 1 {
		t.Fatalf("after reset attempts=%d, want 1", attempts)
	}
	_ = gmToken
}

func TestF1RateLimitPerSessionNotGlobal(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sidA, _ := f1CreateSession(t, srv, gmToken)
	sidB, codeB := f1CreateSession(t, srv, gmToken)
	// 10 wrong against A
	for i := 0; i < 10; i++ {
		r, err := http.Post(srv.URL+"/api/v1/sessions/"+sidA+"/join", "application/json", bytes.NewBufferString(`{"code":"ZZZZZZ"}`))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
	// B first attempt not limited and can succeed with correct code
	r, err := http.Post(srv.URL+"/api/v1/sessions/"+sidB+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, codeB)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("session B blocked by A: status %d", r.StatusCode)
	}
}

func TestF1ForgedSeatTokenBuysNothing(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	// forged token
	req, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+"aaaa.bbbb")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("forged token status %d", resp.StatusCode)
	}
}

func TestF1SeatTokenWrongSessionAnonymous(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sidA, codeA := f1CreateSession(t, srv, gmToken)
	sidB, _ := f1CreateSession(t, srv, gmToken)
	// join A to get seat token
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sidA+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, codeA)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	tok := j["token"].(string)
	// use token on B state endpoint
	req, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sidB, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized && resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong-session seat token status %d, want 401/404", resp2.StatusCode)
	}
}

func TestF1SeatTokenAfterCloseNothing(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, code := f1CreateSession(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	tok := j["token"].(string)
	// close
	cr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	cr.Header.Set("Authorization", "Bearer "+gmToken)
	cresp, err := http.DefaultClient.Do(cr)
	if err != nil {
		t.Fatal(err)
	}
	cresp.Body.Close()
	// state with token
	req, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized && r.StatusCode != http.StatusNotFound {
		t.Fatalf("after close status %d, want 401/404", r.StatusCode)
	}
}

func TestF1CampaignCodeAtSessionJoinRejected(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	// mint campaign join code
	code := f0aMintJoinCode(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("campaign code accepted at session join")
	}
}

func TestF1SessionCodeAtAccountJoinRejected(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	_, sessCode := f1CreateSession(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, sessCode)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("session code accepted at account join")
	}
}

func TestF1RotatedCodeOldRejectedNewWorks(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, oldCode := f1CreateSession(t, srv, gmToken)
	// rotate
	rr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/code/rotate", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	rr.Header.Set("Authorization", "Bearer "+gmToken)
	rresp, err := http.DefaultClient.Do(rr)
	if err != nil {
		t.Fatal(err)
	}
	defer rresp.Body.Close()
	var rb map[string]any
	json.NewDecoder(rresp.Body).Decode(&rb)
	newCode := rb["code"].(string)
	if newCode == oldCode {
		t.Fatal("code did not rotate")
	}
	// old rejected
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, oldCode)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old code status %d, want 401", resp.StatusCode)
	}
	// new works
	resp2, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, newCode)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("new code status %d, want 200", resp2.StatusCode)
	}
}

func TestF1CloseIdempotent(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	// first close with batch id
	r1, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", bytes.NewBufferString(`{"batch_id":"mybatch"}`))
	if err != nil {
		t.Fatal(err)
	}
	r1.Header.Set("Authorization", "Bearer "+gmToken)
	resp1, err := http.DefaultClient.Do(r1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	var b1 map[string]any
	json.NewDecoder(resp1.Body).Decode(&b1)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("close1 %d: %+v", resp1.StatusCode, b1)
	}
	batch1 := b1["batch_id"].(string)
	closedAt1 := ""
	// get state
	gr, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	if err != nil {
		t.Fatal(err)
	}
	gr.Header.Set("Authorization", "Bearer "+gmToken)
	gresp, err := http.DefaultClient.Do(gr)
	if err != nil {
		t.Fatal(err)
	}
	defer gresp.Body.Close()
	var gs map[string]any
	json.NewDecoder(gresp.Body).Decode(&gs)
	closedAt1, _ = gs["closed_at"].(string)

	// second close
	r2, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", bytes.NewBufferString(`{"batch_id":"other"}`))
	if err != nil {
		t.Fatal(err)
	}
	r2.Header.Set("Authorization", "Bearer "+gmToken)
	resp2, err := http.DefaultClient.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var b2 map[string]any
	json.NewDecoder(resp2.Body).Decode(&b2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("close2 %d: %+v", resp2.StatusCode, b2)
	}
	if b2["batch_id"].(string) != batch1 {
		t.Fatalf("batch changed: %s vs %s", b2["batch_id"], batch1)
	}
	// closed_at frozen
	gr2, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	if err != nil {
		t.Fatal(err)
	}
	gr2.Header.Set("Authorization", "Bearer "+gmToken)
	gresp2, err := http.DefaultClient.Do(gr2)
	if err != nil {
		t.Fatal(err)
	}
	defer gresp2.Body.Close()
	var gs2 map[string]any
	json.NewDecoder(gresp2.Body).Decode(&gs2)
	if gs2["closed_at"].(string) != closedAt1 {
		t.Fatalf("closed_at changed: %s vs %s", gs2["closed_at"], closedAt1)
	}
}

func TestF1JoinAfterCloseError(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, code := f1CreateSession(t, srv, gmToken)
	_ = code
	// close
	cr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	if err != nil {
		t.Fatal(err)
	}
	cr.Header.Set("Authorization", "Bearer "+gmToken)
	cresp, err := http.DefaultClient.Do(cr)
	if err != nil {
		t.Fatal(err)
	}
	cresp.Body.Close()
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(`{"code":"AAAAAA"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), "session_closed") {
		t.Fatalf("no session_closed: %s", string(b))
	}
}

func TestF1SeatsAfterCloseError(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	cr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	if err != nil {
		t.Fatal(err)
	}
	cr.Header.Set("Authorization", "Bearer "+gmToken)
	cresp, err := http.DefaultClient.Do(cr)
	if err != nil {
		t.Fatal(err)
	}
	cresp.Body.Close()
	// add seat
	body := bytes.NewBufferString(`{"kind":"guest"}`)
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/seats", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), "session_closed") {
		t.Fatalf("no session_closed: %s", string(b))
	}
}

func TestF1RotateAfterCloseError(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	cr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	if err != nil {
		t.Fatal(err)
	}
	cr.Header.Set("Authorization", "Bearer "+gmToken)
	cresp, err := http.DefaultClient.Do(cr)
	if err != nil {
		t.Fatal(err)
	}
	cresp.Body.Close()
	rr, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/code/rotate", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(rr)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), "session_closed") {
		t.Fatalf("no session_closed: %s", string(b))
	}
}

func TestF1CloseClientBatchId(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, _ := f1CreateSession(t, srv, gmToken)
	// close with client batch id
	r1, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", bytes.NewBufferString(`{"batch_id":"client-123"}`))
	if err != nil {
		t.Fatal(err)
	}
	r1.Header.Set("Authorization", "Bearer "+gmToken)
	resp1, err := http.DefaultClient.Do(r1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	var b1 map[string]any
	json.NewDecoder(resp1.Body).Decode(&b1)
	if b1["batch_id"].(string) != "client-123" {
		t.Fatalf("batch_id=%v, want client-123", b1["batch_id"])
	}
	// repeat with same batch id returns same
	r2, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", bytes.NewBufferString(`{"batch_id":"client-123"}`))
	if err != nil {
		t.Fatal(err)
	}
	r2.Header.Set("Authorization", "Bearer "+gmToken)
	resp2, err := http.DefaultClient.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var b2 map[string]any
	json.NewDecoder(resp2.Body).Decode(&b2)
	if b2["batch_id"].(string) != "client-123" {
		t.Fatalf("repeat batch=%v", b2["batch_id"])
	}
	// absent -> server generates (new session)
	sid2, _ := f1CreateSession(t, srv, gmToken)
	r3, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid2+"/close", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	r3.Header.Set("Authorization", "Bearer "+gmToken)
	resp3, err := http.DefaultClient.Do(r3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var b3 map[string]any
	json.NewDecoder(resp3.Body).Decode(&b3)
	if !strings.HasPrefix(b3["batch_id"].(string), "batch-") {
		t.Fatalf("generated batch=%v", b3["batch_id"])
	}
}

func TestF1StateEndpointNarrowing(t *testing.T) {
	_, v, srv, gmToken := f1Env(t)
	_ = v
	sid, code := f1CreateSession(t, srv, gmToken)
	// join to get seat token
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	seatTok := j["token"].(string)

	// GM sees all
	gr, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	if err != nil {
		t.Fatal(err)
	}
	gr.Header.Set("Authorization", "Bearer "+gmToken)
	gresp, err := http.DefaultClient.Do(gr)
	if err != nil {
		t.Fatal(err)
	}
	gresp.Body.Close()
	if gresp.StatusCode != http.StatusOK {
		t.Fatalf("GM state %d", gresp.StatusCode)
	}

	// holder sees state
	hr, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid, nil)
	if err != nil {
		t.Fatal(err)
	}
	hr.Header.Set("Authorization", "Bearer "+seatTok)
	hresp, err := http.DefaultClient.Do(hr)
	if err != nil {
		t.Fatal(err)
	}
	hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		t.Fatalf("holder state %d", hresp.StatusCode)
	}

	// anonymous -> 401
	ar, err := http.Get(srv.URL + "/api/v1/sessions/" + sid)
	if err != nil {
		t.Fatal(err)
	}
	ar.Body.Close()
	if ar.StatusCode != http.StatusUnauthorized && ar.StatusCode != http.StatusNotFound {
		t.Fatalf("anon state %d", ar.StatusCode)
	}

	// other-session seat -> 401
	sid2, _ := f1CreateSession(t, srv, gmToken)
	_ = sid2
	orr, err := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid2, nil)
	if err != nil {
		t.Fatal(err)
	}
	orr.Header.Set("Authorization", "Bearer "+seatTok)
	oresp, err := http.DefaultClient.Do(orr)
	if err != nil {
		t.Fatal(err)
	}
	oresp.Body.Close()
	if oresp.StatusCode != http.StatusUnauthorized && oresp.StatusCode != http.StatusNotFound {
		t.Fatalf("other-session seat state %d", oresp.StatusCode)
	}
}

func TestF1ConcurrentJoinRace(t *testing.T) {
	_, _, srv, gmToken := f1Env(t)
	sid, code := f1CreateSession(t, srv, gmToken)
	// pre-add one seat
	f1AddSeat(t, srv, gmToken, sid, "guest", "", "")
	const N = 12
	var wg sync.WaitGroup
	statuses := make(chan int, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
			if err != nil {
				statuses <- 0
				return
			}
			statuses <- resp.StatusCode
			resp.Body.Close()
		}()
	}
	wg.Wait()
	close(statuses)
	ok := 0
	fail := 0
	for st := range statuses {
		if st == http.StatusOK {
			ok++
		} else {
			fail++
		}
	}
	if ok != 1 {
		t.Fatalf("concurrent join: ok=%d fail=%d, want 1 ok", ok, fail)
	}
	_ = gmToken
}

func TestF1AuthStoreUnregisteredFailClosed(t *testing.T) {
	dir := t.TempDir()
	if err := runInit([]string{dir}); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	tk := tokenKeyFor(v)
	// issue a seat token
	tok, _ := issueToken(tk, tokenClaims{Kind: "seat", ID: "seat1", Bound: "c", Session: "s1"})
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+tok)
	p := principalFromRequest(r, tk)
	if p.IsSeat() {
		t.Fatal("seat principal accepted with no registered store")
	}
}
