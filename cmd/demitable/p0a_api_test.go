package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// P0a (phase-5-plan §3): api.md staleness pass. Each test below is a
// committed check that fails if the code and the (rewritten) doc disagree
// on route existence, WS type emission, or intent handling. They are
// deliberately behavioural or grep-based so that a doc/code drift breaks CI.

// --- route existence: POST /api/v1/compute (Gap 3) -------------------------

func TestP0aComputeRouteExists(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/compute",
		bytes.NewBufferString(`{"formulas":["max_hp"],"context":{"attributes":{"rating":3}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/v1/compute not routed: status %d", resp.StatusCode)
	}
	// Empty formulas evaluate the pack's full formula set -> 200.
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(b))
	}
}

// --- ?expand=1 is accepted and echoed, never gates computation (Gap 3) -----

func TestP0aExpandEchoedOnCharacter(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	defer srv.Close()

	// create a character
	payload := map[string]any{"system": "example", "name": "ExpandChar"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	uid := created["uid"].(string)

	for _, tc := range []struct {
		q    string
		want bool
	}{
		{"?expand=1", true},
		{"", false},
	} {
		r, _ := http.NewRequest("GET", srv.URL+"/api/v1/characters/"+uid+tc.q, nil)
		r.Header.Set("Authorization", "Bearer "+gmToken)
		rr, _ := http.DefaultClient.Do(r)
		var sheet map[string]any
		json.NewDecoder(rr.Body).Decode(&sheet)
		rr.Body.Close()
		got, _ := sheet["expand"].(bool)
		if got != tc.want {
			t.Fatalf("expand=%v want %v (q=%q)", got, tc.want, tc.q)
		}
		// computed is present regardless of expand
		if _, ok := sheet["computed"]; !ok {
			t.Fatalf("computed missing (q=%q)", tc.q)
		}
	}
}

// --- sessions header no longer claims "none implemented" -------------------

func TestP0aApiMdSessionsHeaderUpdated(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "none of these endpoints are implemented in F0") {
		t.Fatal("stale sessions header still present in api.md")
	}
	if !strings.Contains(s, "shipped in phase 4") {
		t.Fatal("sessions header not updated to shipped state")
	}
}

// --- Gap 2/3 stale futures removed ----------------------------------------

func TestP0aApiMdNoStaleFutures(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	stale := []string{
		"does not exist yet — it is F0 scope",
		"webapp spends no credential yet (F5)",
		"receives a full `session.state`",
	}
	for _, ph := range stale {
		if strings.Contains(s, ph) {
			t.Fatalf("stale phrase still present: %q", ph)
		}
	}
}

// --- WS: the three advertised intents are ignored (finding 14) -------------

func TestP0aWSIntentsIgnoredInCode(t *testing.T) {
	b, err := os.ReadFile("websocket.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, intent := range []string{`"session.command"`, `"roll.request"`, `"note.command"`} {
		if strings.Contains(s, "case "+intent+":") {
			t.Fatalf("intent %s is handled but doc marks it ignored: %s", intent, "case "+intent+":")
		}
	}
}

// --- WS: session.state / session.patch are never emitted (cut in F5) -------

func TestP0aNoSessionStateEmission(t *testing.T) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e, "_test.go") {
			continue
		}
		b, err := os.ReadFile(e)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, typ := range []string{`"session.state"`, `"session.patch"`} {
			if strings.Contains(s, typ) {
				t.Fatalf("%s emits/cites %s; doc says cut", e, typ)
			}
		}
	}
}

// --- WS reconnect: no full session.state push ------------------------------

func TestP0aWSReconnectNoFullState(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nhi\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1"})

	conn := dialWSN7(t, srv, tok)
	defer conn.Close()

	// first frame must be hello
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	payload, err := readFrameN7(conn)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	json.Unmarshal(payload, &env)
	if env["type"] != "hello" {
		t.Fatalf("first frame %v want hello", env["type"])
	}

	// no further frame (in particular no session.state) should arrive
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, err = readFrameN7(conn)
	if err == nil {
		t.Fatal("unexpected second frame on connect; expected none (session.state cut)")
	}
}

// --- WS intents ignored: ensure doc marks them ignored ---------------------

func TestP0aApiMdWsIntentsMarkedIgnored(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, ph := range []string{"`session.command` | **ignored.", "`roll.request` | **ignored.", "`note.command` | **ignored."} {
		if !strings.Contains(s, ph) {
			t.Fatalf("api.md does not mark %s as ignored", ph)
		}
	}
}

// --- helper: silence unused imports if needed ------------------------------

var _ = bufio.NewReader
var _ = url.Parse
var _ = net.Dial
var _ = time.Second
var _ = fmt.Sprintf
var _ = http.MethodGet
