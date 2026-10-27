package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func f0aSetup(t *testing.T) (string, *httptest.Server, string, string) {
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
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm auth status %d: %+v", resp.StatusCode, body)
	}
	token := body["token"].(string)
	return dir, srv, gmSecret, token
}

func f0aMintJoinCode(t *testing.T, srv *httptest.Server, gmToken string) string {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"player1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var pc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&pc); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mint player status %d: %+v", resp.StatusCode, pc)
	}
	return pc["join_code"].(string)
}

func TestF0aJoinSetPassphraseLoginRoundTrip(t *testing.T) {
	dir, srv, _, gmToken := f0aSetup(t)
	_ = dir
	code := f0aMintJoinCode(t, srv, gmToken)

	// Step 1: join redeems the code. No passphrase is set yet.
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Body.Close()
	var j map[string]any
	json.NewDecoder(r1.Body).Decode(&j)
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("join status %d: %+v", r1.StatusCode, j)
	}
	player := j["player"].(string)
	joinToken := j["token"].(string)
	if player == "" || joinToken == "" {
		t.Fatalf("join missing player/token: %+v", j)
	}

	// Step 2: set a passphrase only the player knows.
	body := map[string]any{"passphrase": "correct horse battery staple"}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/set-passphrase", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+joinToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sp map[string]any
	json.NewDecoder(resp.Body).Decode(&sp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set-passphrase status %d: %+v", resp.StatusCode, sp)
	}

	// Step 3: login with the chosen passphrase succeeds.
	lr, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "correct horse battery staple")))
	if err != nil {
		t.Fatal(err)
	}
	defer lr.Body.Close()
	var lb map[string]any
	json.NewDecoder(lr.Body).Decode(&lb)
	if lr.StatusCode != http.StatusOK {
		t.Fatalf("login status %d: %+v", lr.StatusCode, lb)
	}
	if lb["token"] == nil || lb["token"] == "" {
		t.Fatalf("login missing token: %+v", lb)
	}

	// The consumed code is NOT a passphrase. Login with the code must fail.
	bad, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("code-as-passphrase login: got %d want 401", bad.StatusCode)
	}
}

func TestF0aJoinConsumedCodeRejected(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)

	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("first join status %d", r1.StatusCode)
	}
	r2, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	var b2 map[string]any
	json.NewDecoder(r2.Body).Decode(&b2)
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reuse consumed code status %d want 401", r2.StatusCode)
	}
}

func TestF0aJoinExpiredCodeRejected(t *testing.T) {
	dir, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)
	codes := loadJoinCodes(dir)
	for i := range codes {
		if codes[i].Code == code {
			codes[i].ExpiresAt = time.Now().Add(-1 * time.Hour)
		}
	}
	saveJoinCodes(dir, codes)
	r, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var b map[string]any
	json.NewDecoder(r.Body).Decode(&b)
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired code status %d want 401: %+v", r.StatusCode, b)
	}
}

func TestF0aLoginWrongPassphraseRejectedAndRateLimited(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	var j map[string]any
	json.NewDecoder(r1.Body).Decode(&j)
	r1.Body.Close()
	player := j["player"].(string)
	joinToken := j["token"].(string)
	// set passphrase
	bb, _ := json.Marshal(map[string]any{"passphrase": "secret-pass"})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/set-passphrase", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+joinToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// first wrong passphrase -> 401
	w1, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "wrong")))
	if err != nil {
		t.Fatal(err)
	}
	w1.Body.Close()
	if w1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong passphrase status %d want 401", w1.StatusCode)
	}
	// second immediate attempt -> rate limited (429)
	w2, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "wrong2")))
	if err != nil {
		t.Fatal(err)
	}
	w2.Body.Close()
	if w2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rate limit status %d want 429", w2.StatusCode)
	}
}

func TestF0aLoginDirect(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	var j map[string]any
	json.NewDecoder(r1.Body).Decode(&j)
	r1.Body.Close()
	player := j["player"].(string)
	joinToken := j["token"].(string)

	// no passphrase set yet -> login fails
	l0, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "anything")))
	if err != nil {
		t.Fatal(err)
	}
	l0.Body.Close()
	if l0.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login before passphrase status %d want 401", l0.StatusCode)
	}
	time.Sleep(2100 * time.Millisecond)

	// set passphrase
	bb, _ := json.Marshal(map[string]any{"passphrase": "only-player-knows"})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/set-passphrase", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+joinToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set-passphrase status %d", resp.StatusCode)
	}

	// correct login
	l1, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "only-player-knows")))
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Body.Close()
	var lb map[string]any
	json.NewDecoder(l1.Body).Decode(&lb)
	if l1.StatusCode != http.StatusOK {
		t.Fatalf("correct login status %d: %+v", l1.StatusCode, lb)
	}
	if lb["player"] != player {
		t.Fatalf("login player = %v want %v", lb["player"], player)
	}
}

func TestF0aGMSecretRotationInvalidatesSessions(t *testing.T) {
	_, srv, gmSecret, oldBearer := f0aSetup(t)
	_ = gmSecret

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/gm/rotate", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+oldBearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate status %d", resp.StatusCode)
	}

	// old bearer now invalid: rotate again with old bearer -> 401
	req3, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/gm/rotate", bytes.NewBufferString(`{}`))
	req3.Header.Set("Authorization", "Bearer "+oldBearer)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old bearer after rotation status %d want 401", resp3.StatusCode)
	}
}

func TestF0aDisjointCodeSpaces(t *testing.T) {
	_, srv, gmSecret, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)

	// campaign join code presented to GM endpoint -> rejected
	r1, err := http.Post(srv.URL+"/api/v1/auth/gm", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"secret":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	r1.Body.Close()
	if r1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("join code as GM secret status %d want 401", r1.StatusCode)
	}
	time.Sleep(4500 * time.Millisecond)
	// GM secret presented to join endpoint -> rejected
	r2, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, gmSecret)))
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GM secret as join code status %d want 401", r2.StatusCode)
	}
	time.Sleep(4500 * time.Millisecond)
	// a short 6-char code presented to join endpoint -> rejected
	r3, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(`{"code":"ABCDEF"}`))
	if err != nil {
		t.Fatal(err)
	}
	r3.Body.Close()
	if r3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("short code as join code status %d want 401", r3.StatusCode)
	}
	// code length check: join codes are 9 chars (xxxx-xxxx)
	if len(code) != 9 {
		t.Fatalf("join code length = %d want 9", len(code))
	}
}

func TestF0aEmptyPassphraseRejected(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	var j map[string]any
	json.NewDecoder(r1.Body).Decode(&j)
	r1.Body.Close()
	player := j["player"].(string)
	joinToken := j["token"].(string)

	// empty passphrase via set-passphrase rejected
	bb, _ := json.Marshal(map[string]any{"passphrase": ""})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/set-passphrase", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+joinToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty passphrase status %d want 400", resp.StatusCode)
	}
	time.Sleep(2100 * time.Millisecond)
	// login with empty passphrase rejected
	lr, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, player, "")))
	if err != nil {
		t.Fatal(err)
	}
	lr.Body.Close()
	if lr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("empty passphrase login status %d want 401", lr.StatusCode)
	}
}
