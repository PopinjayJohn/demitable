package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func f3env(t *testing.T) (*vault.Vault, *httptest.Server, string, string) {
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
	gmTok := body["token"].(string)
	if gmTok == "" {
		t.Fatal("gm auth returned empty token")
	}
	return v, srv, gmTok, dir
}

func f3createSession(t *testing.T, srv *httptest.Server, gmToken string) (string, string) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/sessions", bytes.NewBufferString(`{"name":"s","campaign":"demo"}`))
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
	id, _ := s["id"].(string)
	code, _ := s["code"].(string)
	if id == "" || code == "" {
		t.Fatalf("create session missing id/code: %+v", s)
	}
	return id, code
}

func f3addSeat(t *testing.T, srv *httptest.Server, gmToken, sid, kind, char, account string) map[string]any {
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
	if out["seat"] == nil {
		t.Fatalf("add seat response missing seat: %+v", out)
	}
	return out["seat"].(map[string]any)
}

func f3joinByCode(t *testing.T, srv *httptest.Server, sid, code string) (string, map[string]any) {
	t.Helper()
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
	tok, _ := j["token"].(string)
	if tok == "" {
		t.Fatal("join returned empty token")
	}
	seat := map[string]any{}
	if s, ok := j["seat"].(map[string]any); ok {
		seat = s
	}
	return tok, seat
}

func f3writeNote(t *testing.T, dir string, v *vault.Vault, content string) {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("tmp_%d.md", time.Now().UnixNano()))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
}

func f3getNoteBody(t *testing.T, srv *httptest.Server, tok, uid string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/"+uid, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err == nil {
		if c, ok := out["content"].(string); ok {
			return resp.StatusCode, c
		}
		if b, ok := out["body"].(string); ok {
			return resp.StatusCode, b
		}
	}
	return resp.StatusCode, string(body)
}

func f3listNotes(t *testing.T, srv *httptest.Server, tok string) (int, []map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return resp.StatusCode, nil
	}
	items := []map[string]any{}
	if it, ok := out["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	return resp.StatusCode, items
}

func TestF3ChatAudienceSeparation(t *testing.T) {
	v, srv, gmTok, dir := f3env(t)
	_ = v
	_ = dir
	_ = gmTok

	tk := tokenKeyFor(v)
	aliceTok, err := issueToken(tk, tokenClaims{Kind: "account", ID: "alice", Campaigns: []string{"demo"}})
	if err != nil || aliceTok == "" {
		t.Fatal("issue alice token")
	}
	bobTok, err := issueToken(tk, tokenClaims{Kind: "account", ID: "bob", Campaigns: []string{"demo"}})
	if err != nil || bobTok == "" {
		t.Fatal("issue bob token")
	}

	// Alice sends owner-visibility message
	body1 := map[string]any{
		"type":       "message",
		"name":       "m1",
		"visibility": "owner",
		"player":     "alice",
		"campaign":   "demo",
		"content":    "rareterm42 alice secret owner msg",
	}
	b1, _ := json.Marshal(body1)
	req1, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewReader(b1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+aliceTok)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK && resp1.StatusCode != http.StatusCreated {
		rb, _ := io.ReadAll(resp1.Body)
		t.Fatalf("create owner msg status %d: %s", resp1.StatusCode, string(rb))
	}
	var c1 map[string]any
	json.NewDecoder(resp1.Body).Decode(&c1)
	uid1, _ := c1["uid"].(string)
	if uid1 == "" {
		t.Fatalf("no uid for created message: %+v", c1)
	}

	// Alice reads 200 with bytes
	stA, bodyA := f3getNoteBody(t, srv, aliceTok, uid1)
	if stA != http.StatusOK || !strings.Contains(bodyA, "rareterm42 alice secret owner msg") {
		t.Fatalf("alice read owner msg: %d %s", stA, bodyA)
	}
	// Bob reads 404 with zero owner bytes
	stB, bodyB := f3getNoteBody(t, srv, bobTok, uid1)
	if stB != http.StatusNotFound || strings.Contains(bodyB, "rareterm42 alice secret owner msg") {
		t.Fatalf("bob read owner msg: %d %s", stB, bodyB)
	}

	// Alice sends party message
	body2 := map[string]any{
		"type":       "message",
		"name":       "m2",
		"visibility": "party",
		"campaign":   "demo",
		"content":    "party hello",
	}
	b2, _ := json.Marshal(body2)
	req2, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewReader(b2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+aliceTok)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var c2 map[string]any
	json.NewDecoder(resp2.Body).Decode(&c2)
	uid2, _ := c2["uid"].(string)
	if uid2 == "" {
		t.Fatalf("no uid for party msg: %+v", c2)
	}
	stA2, bodyA2 := f3getNoteBody(t, srv, aliceTok, uid2)
	stB2, bodyB2 := f3getNoteBody(t, srv, bobTok, uid2)
	if stA2 != http.StatusOK || stB2 != http.StatusOK {
		t.Fatalf("party msg read status: %d %d", stA2, stB2)
	}
	if !strings.Contains(bodyA2, "party hello") || !strings.Contains(bodyB2, "party hello") {
		t.Fatalf("party msg bytes missing")
	}

	// O2 search for a rare term
	reqSA, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes?q="+url.QueryEscape(`body contains "rareterm42"`), nil)
	reqSA.Header.Set("Authorization", "Bearer "+aliceTok)
	respSA, _ := http.DefaultClient.Do(reqSA)
	bodySA, _ := io.ReadAll(respSA.Body)
	respSA.Body.Close()
	if !strings.Contains(string(bodySA), uid1) {
		t.Fatalf("alice should find owner message: %s", string(bodySA))
	}
	reqSB, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes?q="+url.QueryEscape(`body contains "rareterm42"`), nil)
	reqSB.Header.Set("Authorization", "Bearer "+bobTok)
	respSB, _ := http.DefaultClient.Do(reqSB)
	bodySB, _ := io.ReadAll(respSB.Body)
	respSB.Body.Close()
	if strings.Contains(string(bodySB), uid1) || strings.Contains(string(bodySB), "rareterm42 alice secret owner msg") {
		t.Fatalf("bob must never find alice's owner message: %s", string(bodySB))
	}
}

func TestF3GuestBoundsProper(t *testing.T) {
	v, srv, gmTok, dir := f3env(t)
	if err := os.WriteFile(filepath.Join(dir, "c1.md"), []byte("---\ntype: character\nuid: c1\nname: C1\ncampaign: \"[[demo]]\"\nvisibility: party\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c2.md"), []byte("---\ntype: character\nuid: c2\nname: C2\ncampaign: \"[[demo]]\"\nvisibility: party\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	sid, code := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid, "guest", "c1", "")
	f3addSeat(t, srv, gmTok, sid, "guest", "c2", "")
	tokG1, _ := f3joinByCode(t, srv, sid, code)
	tokG2, _ := f3joinByCode(t, srv, sid, code)
	if tokG1 == "" || tokG2 == "" {
		t.Fatal("empty guest token")
	}

	st1, _ := f3getNoteBody(t, srv, tokG1, "c1")
	st2c, _ := f3getNoteBody(t, srv, tokG2, "c2")
	if st1 != http.StatusOK || st2c != http.StatusOK {
		t.Fatalf("own bound character: %d %d", st1, st2c)
	}
	st1b, _ := f3getNoteBody(t, srv, tokG1, "c2")
	st2b, _ := f3getNoteBody(t, srv, tokG2, "c1")
	if st1b != http.StatusNotFound || st2b != http.StatusNotFound {
		t.Fatalf("other bound character should be 404: %d %d", st1b, st2b)
	}
	f3writeNote(t, dir, v, "---\ntype: note\nuid: pn1\nname: PN1\ncampaign: demo\nvisibility: party\n---\nparty\n")
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	stp1, _ := f3getNoteBody(t, srv, tokG1, "pn1")
	if stp1 != http.StatusNotFound {
		t.Fatalf("party campaign note should be 404 for guest with empty campaigns: %d", stp1)
	}
}

func TestF3NonMemberChannelRead(t *testing.T) {
	v, srv, gmTok, dir := f3env(t)
	sid, code := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid, "guest", "", "")
	tokM, _ := f3joinByCode(t, srv, sid, code)
	_ = tokM
	f3writeNote(t, dir, v, "---\ntype: note\nuid: pn2\nname: PN2\ncampaign: demo\nvisibility: party\n---\nparty\n")
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	sid2, code2 := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid2, "guest", "", "")
	tokN2, _ := f3joinByCode(t, srv, sid2, code2)
	if tokN2 == "" {
		t.Fatal("empty guest token")
	}
	st, body := f3getNoteBody(t, srv, tokN2, "pn2")
	if st != http.StatusNotFound || strings.Contains(body, "party") {
		t.Fatalf("non-member channel read should be 404 with no byte leak: %d %s", st, body)
	}
	stl, items := f3listNotes(t, srv, tokN2)
	if stl != http.StatusOK {
		t.Fatalf("list status %d", stl)
	}
	for _, it := range items {
		if u, _ := it["uid"].(string); u == "pn2" {
			t.Fatalf("list must exclude non-member note")
		}
	}
}

func TestF3DisplayBindingAndState(t *testing.T) {
	v, srv, gmTok, dir := f3env(t)
	_ = v
	_ = dir
	sid, _ := f3createSession(t, srv, gmTok)
	// bind display twice - must return identical token
	reqb1, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/display/bind", nil)
	reqb1.Header.Set("Authorization", "Bearer "+gmTok)
	respb1, _ := http.DefaultClient.Do(reqb1)
	var db1 map[string]any
	json.NewDecoder(respb1.Body).Decode(&db1)
	respb1.Body.Close()
	tok1 := ""
	if s, ok := db1["token"].(string); ok {
		tok1 = s
	} else if s, ok := db1["display_token"].(string); ok {
		tok1 = s
	}
	time.Sleep(1100 * time.Millisecond)
	reqb2, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/display/bind", nil)
	reqb2.Header.Set("Authorization", "Bearer "+gmTok)
	respb2, _ := http.DefaultClient.Do(reqb2)
	var db2 map[string]any
	json.NewDecoder(respb2.Body).Decode(&db2)
	respb2.Body.Close()
	tok2 := ""
	if s, ok := db2["token"].(string); ok {
		tok2 = s
	} else if s, ok := db2["display_token"].(string); ok {
		tok2 = s
	}
	if tok1 == "" || tok2 == "" || tok1 != tok2 {
		t.Fatalf("double-bind must return identical token: %q %q", tok1, tok2)
	}

	// party note visible on display state
	f3writeNote(t, dir, v, "---\ntype: note\nuid: pnote\nname: PNote\ncampaign: demo\nvisibility: party\n---\npbody\n")
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	reqd, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/display/state", nil)
	reqd.Header.Set("Authorization", "Bearer "+tok1)
	respd, _ := http.DefaultClient.Do(reqd)
	var ds map[string]any
	json.NewDecoder(respd.Body).Decode(&ds)
	respd.Body.Close()
	items := []map[string]any{}
	if it, ok := ds["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	foundParty := false
	for _, it := range items {
		if u, _ := it["uid"].(string); u == "pnote" {
			foundParty = true
			break
		}
	}
	if !foundParty {
		t.Fatalf("party note visible on display state: %+v", ds)
	}

	// owner-scoped note -> absent
	f3writeNote(t, dir, v, "---\ntype: note\nuid: onote\nname: ONote\ncampaign: demo\nvisibility: owner\nplayer: p1\n---\nobody\n")
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	reqd2, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/display/state", nil)
	reqd2.Header.Set("Authorization", "Bearer "+tok1)
	respd2, _ := http.DefaultClient.Do(reqd2)
	var ds2 map[string]any
	json.NewDecoder(respd2.Body).Decode(&ds2)
	respd2.Body.Close()
	items2 := []map[string]any{}
	if it, ok := ds2["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items2 = append(items2, m)
			}
		}
	}
	for _, m := range items2 {
		if u, _ := m["uid"].(string); u == "onote" {
			t.Fatalf("owner note must be absent from display")
		}
	}

	// granted-to-one note -> absent
	f3writeNote(t, dir, v, "---\ntype: note\nuid: gnote\nname: GNote\ncampaign: demo\nvisibility: discovered\ngrants:\n  - p1\n---\ng\n")
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	reqd3, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/display/state", nil)
	reqd3.Header.Set("Authorization", "Bearer "+tok1)
	respd3, _ := http.DefaultClient.Do(reqd3)
	var ds3 map[string]any
	json.NewDecoder(respd3.Body).Decode(&ds3)
	respd3.Body.Close()
	items3 := []map[string]any{}
	if it, ok := ds3["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items3 = append(items3, m)
			}
		}
	}
	for _, m := range items3 {
		if u, _ := m["uid"].(string); u == "gnote" {
			t.Fatalf("granted-to-one note must be absent from display")
		}
	}

	// discovered-by-one note -> absent
	f3writeNote(t, dir, v, "---\ntype: note\nuid: dnote\nname: DNote\ncampaign: demo\nvisibility: discovered\ndiscovered:\n  - p1\n---\nd\n")
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	reqd4, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/display/state", nil)
	reqd4.Header.Set("Authorization", "Bearer "+tok1)
	respd4, _ := http.DefaultClient.Do(reqd4)
	var ds4 map[string]any
	json.NewDecoder(respd4.Body).Decode(&ds4)
	respd4.Body.Close()
	items4 := []map[string]any{}
	if it, ok := ds4["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items4 = append(items4, m)
			}
		}
	}
	for _, m := range items4 {
		if u, _ := m["uid"].(string); u == "dnote" {
			t.Fatalf("discovered-by-one note must be absent from display")
		}
	}

	// seat token on display state -> 401 (binds-no-seat)
	sid2, code2 := f3createSession(t, srv, gmTok)
	f3addSeat(t, srv, gmTok, sid2, "guest", "", "")
	tokSeat2, _ := f3joinByCode(t, srv, sid2, code2)
	if tokSeat2 == "" {
		t.Fatal("empty seat token")
	}
	reqds, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid2+"/display/state", nil)
	reqds.Header.Set("Authorization", "Bearer "+tokSeat2)
	respds, _ := http.DefaultClient.Do(reqds)
	if respds.StatusCode != http.StatusUnauthorized {
		t.Fatalf("seat token on display state should be 401: %d", respds.StatusCode)
	}
	respds.Body.Close()
}

func TestF3RevealTargetsViaWS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "r.md"), []byte("---\ntype: note\nuid: rn1\nname: RN1\ncampaign: demo\nvisibility: party\n---\n> [!secret] s1\n> hidden1 ^blk1\n\n> [!secret] s2\n> hidden2 ^blk2\n\n> [!secret] s3\n> hidden3 ^blk3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1"})
	p2Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p2"})
	outTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p3"})
	dispTok, _ := issueToken(tk, tokenClaims{Kind: "display", Campaigns: []string{"demo"}})
	if gmTok == "" || p1Tok == "" || p2Tok == "" || outTok == "" || dispTok == "" {
		t.Fatal("empty token")
	}

	pc1 := dialWSN7(t, srv, p1Tok)
	defer pc1.Close()
	readFrameN7(pc1) // hello
	pc2 := dialWSN7(t, srv, p2Tok)
	defer pc2.Close()
	readFrameN7(pc2) // hello
	pco := dialWSN7(t, srv, outTok)
	defer pco.Close()
	readFrameN7(pco) // hello
	pcd := dialWSN7(t, srv, dispTok)
	defer pcd.Close()
	readFrameN7(pcd) // hello

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)

	// to one player
	msg, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{
			"note_uid": "rn1",
			"block_id": "blk1",
			"audience": map[string]any{"players": []string{"p1"}},
		},
	})
	writeFrameN7(gc, msg)
	pc1.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := readFrameN7(pc1); err != nil {
		t.Fatalf("p1 should see reveal: %v", err)
	}
	pc2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := readFrameN7(pc2); err == nil {
		t.Fatalf("p2 should not see reveal to p1 only")
	}
	pc2.SetReadDeadline(time.Time{})
	pco.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := readFrameN7(pco); err == nil {
		t.Fatalf("outsider should not see reveal to p1 only")
	}
	pco.SetReadDeadline(time.Time{})

	// to display
	msg2, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{
			"note_uid": "rn1",
			"block_id": "blk2",
			"audience": map[string]any{"display": true},
		},
	})
	writeFrameN7(gc, msg2)
	pcd.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := readFrameN7(pcd); err != nil {
		t.Fatalf("display should see reveal: %v", err)
	}
	// http body check: display sees block, players do not
	time.Sleep(200 * time.Millisecond)
	stD, bodyD := f3getNoteBody(t, srv, dispTok, "rn1")
	if stD != http.StatusOK || !strings.Contains(bodyD, "hidden2") {
		t.Fatalf("display should see revealed block: %d %s", stD, bodyD)
	}
	stP1, bodyP1 := f3getNoteBody(t, srv, p1Tok, "rn1")
	if stP1 == http.StatusOK && strings.Contains(bodyP1, "hidden2") {
		t.Fatalf("p1 should not see display-targeted block")
	}

	// to all
	msg3, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{
			"note_uid": "rn1",
			"block_id": "blk3",
			"audience": map[string]any{"all": true},
		},
	})
	writeFrameN7(gc, msg3)
	pc1.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := readFrameN7(pc1); err != nil {
		t.Fatalf("p1 should see reveal to all: %v", err)
	}
	pc2.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := readFrameN7(pc2); err != nil {
		t.Fatalf("p2 should see reveal to all: %v", err)
	}
	pco.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := readFrameN7(pco); err != nil {
		t.Fatalf("outsider should see reveal to all: %v", err)
	}
	pco.SetReadDeadline(time.Time{})

	// journal holds exactly 3 reveal entries for the note
	reqj, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	reqj.Header.Set("Authorization", "Bearer "+gmTok)
	respj, _ := http.DefaultClient.Do(reqj)
	var j map[string]any
	json.NewDecoder(respj.Body).Decode(&j)
	respj.Body.Close()
	items := []map[string]any{}
	if it, ok := j["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	count := 0
	for _, it := range items {
		if op, _ := it["op"].(string); op == "reveal" {
			if uid, _ := it["uid"].(string); uid == "rn1" {
				count++
			}
		}
	}
	if count != 3 {
		t.Fatalf("journal must hold exactly 3 reveal entries for note, got %d: %+v", count, items)
	}
}

func TestF3ChatDeleteRevert(t *testing.T) {
	v, srv, gmTok, dir := f3env(t)
	_ = dir
	_ = v
	_, _ = f3createSession(t, srv, gmTok)
	body := map[string]any{
		"type":       "message",
		"name":       "msgdel",
		"visibility": "party",
		"campaign":   "demo",
		"player":     "p1",
		"content":    "hello world",
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewReader(b))
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
	tk := tokenKeyFor(v)
	pTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})
	if pTok == "" {
		t.Fatal("empty player token")
	}
	reqd, _ := http.NewRequest("DELETE", srv.URL+"/api/v1/notes/"+uid, nil)
	reqd.Header.Set("Authorization", "Bearer "+pTok)
	respd, _ := http.DefaultClient.Do(reqd)
	respd.Body.Close()
	st, bodyr := f3getNoteBody(t, srv, pTok, uid)
	if st != http.StatusNotFound {
		t.Fatalf("after delete owner should get 404: %d %s", st, bodyr)
	}
	reqj, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	reqj.Header.Set("Authorization", "Bearer "+gmTok)
	respj, _ := http.DefaultClient.Do(reqj)
	var j map[string]any
	json.NewDecoder(respj.Body).Decode(&j)
	respj.Body.Close()
	delID := ""
	if it, ok := j["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				if op, _ := m["op"].(string); op == "delete" {
					if u, _ := m["uid"].(string); u == uid {
						if id, _ := m["id"].(string); id != "" {
							delID = id
							break
						}
					}
				}
			}
		}
	}
	if delID == "" {
		t.Fatalf("delete journal entry must exist")
	}
	reqr, _ := http.NewRequest("POST", srv.URL+"/api/v1/journal/"+delID+"/revert", nil)
	reqr.Header.Set("Authorization", "Bearer "+gmTok)
	respr, _ := http.DefaultClient.Do(reqr)
	if respr.StatusCode != http.StatusOK {
		t.Fatalf("revert status %d", respr.StatusCode)
	}
	respr.Body.Close()
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	st2, bodyr2 := f3getNoteBody(t, srv, pTok, uid)
	if st2 != http.StatusOK || !strings.Contains(bodyr2, "hello world") {
		t.Fatalf("after revert should be 200 with bytes restored: %d %s", st2, bodyr2)
	}
}

func TestF3CreateNoteVocabularyParity(t *testing.T) {
	_, srv, gmTok, _ := f3env(t)
	body := map[string]any{
		"type":       "message",
		"name":       "bad",
		"visibility": "Party",
		"campaign":   "demo",
		"content":    "hi",
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/notes", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmTok)
	resp, _ := http.DefaultClient.Do(req)
	bodyr, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid visibility should be rejected with 400: %d %s", resp.StatusCode, string(bodyr))
	}
	if !strings.Contains(string(bodyr), "error") {
		t.Fatalf("rejected response must have named error: %s", string(bodyr))
	}
}

func TestF3DisplayGatePinned(t *testing.T) {
	disp := vault.Principal{Kind: "display", ID: "p1", Bound: "bound1", Discovered: []string{"dn1"}}
	acct := vault.Account("p1", "demo")
	acct.Discovered = []string{"dn1"}

	granted := vault.Note{UID: "gn1", Name: "GN", Campaign: "demo", Visibility: "discovered", Grants: []string{"p1"}}
	if !vault.Visible(acct, granted) {
		t.Fatalf("account should see granted note")
	}
	if vault.Visible(disp, granted) {
		t.Fatalf("display must not see granted note")
	}

	owner := vault.Note{UID: "on1", Name: "ON", Campaign: "demo", Visibility: "owner", Player: "p1"}
	if !vault.Visible(acct, owner) {
		t.Fatalf("account should see owner note")
	}
	if vault.Visible(disp, owner) {
		t.Fatalf("display must not see owner note")
	}

	discovered := vault.Note{UID: "dn1", Name: "DN", Campaign: "demo", Visibility: "discovered"}
	if !vault.Visible(acct, discovered) {
		t.Fatalf("account should see discovered note")
	}
	if vault.Visible(disp, discovered) {
		t.Fatalf("display must not see discovered note")
	}
}
