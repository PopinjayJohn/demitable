package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
	"time"
)

// getSessionStateREST fetches the REST session-state document for a principal.
func getSessionStateREST(t *testing.T, srv *httptest.Server, token, id string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET session %d: %s", resp.StatusCode, string(body))
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func addSeatForBroadcast(t *testing.T, srv *httptest.Server, gmToken, sid string) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/seats", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add seat %d", resp.StatusCode)
	}
}

// TestSessionStateEmissionConvergesToREST pins that the WS session.state and
// session.patch frames carry the same payload as GET /sessions/:id, for both
// GM and seat principals.
func TestSessionStateEmissionConvergesToREST(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmToken)

	// GM principal.
	gc := dialWSN7(t, srv, gmToken)
	defer gc.Close()
	if _, err := readFrameN7(gc); err != nil {
		t.Fatal(err)
	}
	addSeatForBroadcast(t, srv, gmToken, sid)

	gc.SetReadDeadline(time.Now().Add(5 * time.Second))
	var gmState, gmPatch map[string]any
	gotState, gotPatch := false, false
	for i := 0; i < 10 && !(gotState && gotPatch); i++ {
		f, err := readFrameN7(gc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(f, &env)
		if env["type"] == "session.state" {
			gmState = env
			gotState = true
		}
		if env["type"] == "session.patch" {
			gmPatch = env
			gotPatch = true
		}
	}
	if !gotState || !gotPatch {
		t.Fatal("GM connection missing session.state/session.patch frames")
	}
	restGM := getSessionStateREST(t, srv, gmToken, sid)
	if !reflect.DeepEqual(gmState["data"], restGM) {
		t.Fatalf("GM session.state != REST\ngot=%v\nwant=%v", gmState["data"], restGM)
	}
	if !reflect.DeepEqual(gmPatch["data"], restGM) {
		t.Fatalf("GM session.patch != REST")
	}

	// Seat principal.
	jreq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/join", bytes.NewBufferString(`{}`))
	jreq.Header.Set("Authorization", "Bearer "+gmToken)
	jresp, err := http.DefaultClient.Do(jreq)
	if err != nil {
		t.Fatal(err)
	}
	defer jresp.Body.Close()
	var jbody map[string]any
	json.NewDecoder(jresp.Body).Decode(&jbody)
	if jresp.StatusCode != http.StatusOK {
		t.Fatalf("gm join %d: %+v", jresp.StatusCode, jbody)
	}
	seatToken := jbody["token"].(string)

	sc := dialWSN7(t, srv, seatToken)
	defer sc.Close()
	if _, err := readFrameN7(sc); err != nil {
		t.Fatal(err)
	}
	addSeatForBroadcast(t, srv, gmToken, sid)

	sc.SetReadDeadline(time.Now().Add(5 * time.Second))
	var seatState, seatPatch map[string]any
	gotS, gotP := false, false
	for i := 0; i < 10 && !(gotS && gotP); i++ {
		f, err := readFrameN7(sc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(f, &env)
		if env["type"] == "session.state" {
			seatState = env
			gotS = true
		}
		if env["type"] == "session.patch" {
			seatPatch = env
			gotP = true
		}
	}
	if !gotS || !gotP {
		t.Fatal("seat connection missing session.state/session.patch frames")
	}
	restSeat := getSessionStateREST(t, srv, seatToken, sid)
	if !reflect.DeepEqual(seatState["data"], restSeat) {
		t.Fatalf("seat session.state != REST")
	}
	if !reflect.DeepEqual(seatPatch["data"], restSeat) {
		t.Fatalf("seat session.patch != REST")
	}
	_ = v
}

// TestSessionCodeRedactedForNonGM pins that the session code is present for
// GM principals and absent (raw bytes too) for seat/display principals, across
// REST and WS emission.
func TestSessionCodeRedactedForNonGM(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	sid, code := f1CreateSession(t, srv, gmToken)
	_ = v

	restGM := getSessionStateREST(t, srv, gmToken, sid)
	if restGM["code"] != code {
		t.Fatalf("GM REST code %v want %v", restGM["code"], code)
	}

	jreq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/join", bytes.NewBufferString(`{}`))
	jreq.Header.Set("Authorization", "Bearer "+gmToken)
	jresp, err := http.DefaultClient.Do(jreq)
	if err != nil {
		t.Fatal(err)
	}
	defer jresp.Body.Close()
	var jbody map[string]any
	json.NewDecoder(jresp.Body).Decode(&jbody)
	seatToken := jbody["token"].(string)

	restSeat := getSessionStateREST(t, srv, seatToken, sid)
	if restSeat["code"] != "" {
		t.Fatalf("seat REST code %v want empty", restSeat["code"])
	}

	// GM WS frame contains code.
	gc := dialWSN7(t, srv, gmToken)
	defer gc.Close()
	readFrameN7(gc) // hello
	addSeatForBroadcast(t, srv, gmToken, sid)
	gc.SetReadDeadline(time.Now().Add(5 * time.Second))
	gmFound := false
	var gmRaw []byte
	for i := 0; i < 10; i++ {
		f, err := readFrameN7(gc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(f, &env)
		if env["type"] == "session.state" || env["type"] == "session.patch" {
			gmRaw = f
			data, _ := env["data"].(map[string]any)
			if data["code"] == code {
				gmFound = true
			}
			break
		}
	}
	if !gmFound {
		t.Fatal("GM WS frame missing code")
	}
	if !bytes.Contains(gmRaw, []byte(code)) {
		t.Fatal("GM raw frame missing code bytes")
	}

	// Seat WS frame contains no code bytes.
	sc := dialWSN7(t, srv, seatToken)
	defer sc.Close()
	readFrameN7(sc) // hello
	addSeatForBroadcast(t, srv, gmToken, sid)
	sc.SetReadDeadline(time.Now().Add(5 * time.Second))
	seatFound := false
	var seatRaw []byte
	for i := 0; i < 10; i++ {
		f, err := readFrameN7(sc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(f, &env)
		if env["type"] == "session.state" || env["type"] == "session.patch" {
			seatRaw = f
			data, _ := env["data"].(map[string]any)
			if data["code"] == "" {
				seatFound = true
			}
			break
		}
	}
	if !seatFound {
		t.Fatal("seat WS frame code not empty")
	}
	if bytes.Contains(seatRaw, []byte(code)) {
		t.Fatalf("seat raw frame contains code bytes")
	}
}

// TestNotesCreateEmitsCreatedBroadcast pins that POST /api/v1/notes emits a
// vault.changed frame with kind:"created" to the GM.
func TestNotesCreateEmitsCreatedBroadcast(t *testing.T) {
	dir, srv, gmToken := n6Setup(t)
	_ = dir

	gc := dialWSN7(t, srv, gmToken)
	defer gc.Close()
	readFrameN7(gc) // hello

	payload := map[string]any{"type": "note", "name": "CreatedNote"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status %d", resp.StatusCode)
	}

	gc.SetReadDeadline(time.Now().Add(5 * time.Second))
	found := false
	for i := 0; i < 10; i++ {
		frame, err := readFrameN7(gc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(frame, &env)
		if env["type"] == "vault.changed" {
			data, _ := env["data"].(map[string]any)
			if data["kind"] == "created" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("GM connection did not receive vault.changed kind:created for note")
	}
}

// TestMintJoinCodeEndpoint pins the new mint endpoint's contract.
func TestMintJoinCodeEndpoint(t *testing.T) {
	dir, srv, gmSecret, gmToken := f0aSetup(t)
	_ = dir
	_ = gmSecret

	// GM mint -> 200 + expires_at.
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/players/alice/join-code", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm mint %d: %s", resp.StatusCode, string(body))
	}
	var pc map[string]any
	json.Unmarshal(body, &pc)
	if pc["expires_at"] == nil || pc["expires_at"] == "" {
		t.Fatalf("missing expires_at: %+v", pc)
	}
	code := pc["join_code"].(string)
	if code == "" {
		t.Fatal("missing join_code")
	}

	// Non-GM -> 403.
	req2, _ := http.NewRequest("POST", srv.URL+"/api/v1/players/bob/join-code", nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("non-gm status %d want 403", resp2.StatusCode)
	}

	// Minted code joins once.
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r1.Body)
	r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("first join %d", r1.StatusCode)
	}
	r2, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r2.Body)
	r2.Body.Close()
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reuse join %d want 401", r2.StatusCode)
	}

	// Backdated expiry -> 401. Use a fresh server/setup so the earlier
	// reuse-401 assertion does not trip the IP backoff limiter.
	dir2, srv2, _, gmToken2 := f0aSetup(t)
	_ = dir2
	req3, _ := http.NewRequest("POST", srv2.URL+"/api/v1/players/carol/join-code", nil)
	req3.Header.Set("Authorization", "Bearer "+gmToken2)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	var pc3 map[string]any
	json.Unmarshal(body3, &pc3)
	code3 := pc3["join_code"].(string)

	codes := loadJoinCodes(dir2)
	for i := range codes {
		if codes[i].Code == code3 {
			codes[i].ExpiresAt = time.Now().Add(-1 * time.Hour)
		}
	}
	saveJoinCodes(dir2, codes)

	r3, err := http.Post(srv2.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code3)))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r3.Body)
	r3.Body.Close()
	if r3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired join %d want 401", r3.StatusCode)
	}
}

var _ = fmt.Sprintf
var _ = http.MethodGet
var _ = httptest.NewServer
var _ = vault.Open
