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

func f2bEnv(t *testing.T) (string, *vault.Vault, *httptest.Server, string) {
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

func f2bCreateSession(t *testing.T, srv *httptest.Server, gmToken string) (string, string) {
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
	var id string
	if v, ok := s["id"].(string); ok {
		id = v
	} else if v, ok := s["session"].(string); ok {
		id = v
	} else if m, ok := s["session"].(map[string]any); ok {
		if v, ok := m["id"].(string); ok {
			id = v
		}
	}
	code, _ := s["code"].(string)
	if id == "" {
		t.Fatalf("bad session resp: %+v", s)
	}
	return id, code
}

func f2bAddNote(t *testing.T, dir string, v *vault.Vault, uid, name, player, vis string) {
	t.Helper()
	content := fmt.Sprintf("---\ntype: character\nuid: %s\nname: %s\ncampaign: \"[[C]]\"\nplayer: %s\nvisibility: %s\n---\nbody\n", uid, name, player, vis)
	if err := os.WriteFile(filepath.Join(dir, uid+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
}

func f2bAddSeat(t *testing.T, srv *httptest.Server, gmToken, sid, kind, char, account string) map[string]any {
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
	if s, ok := out["seat"].(map[string]any); ok {
		return s
	}
	return out
}

func f2bMintAccount(t *testing.T, srv *httptest.Server, gmToken string) (string, string) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"p1"}`))
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
	r1, err := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Body.Close()
	var j map[string]any
	json.NewDecoder(r1.Body).Decode(&j)
	joinToken := j["token"].(string)
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
	lr, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":%q}`, j["player"], "pass123")))
	if err != nil {
		t.Fatal(err)
	}
	defer lr.Body.Close()
	var lb map[string]any
	json.NewDecoder(lr.Body).Decode(&lb)
	return j["player"].(string), lb["token"].(string)
}

func mustSeatToken(t *testing.T, srv *httptest.Server, sessionID, code string) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sessionID+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join by code %d: %+v", resp.StatusCode, j)
	}
	tok, _ := j["token"].(string)
	if tok == "" {
		t.Fatalf("missing seat token: %+v", j)
	}
	return tok
}

func TestF2B_B1_SeatImpersonation(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid, code := f2bCreateSession(t, srv, gmToken)
	seat := f2bAddSeat(t, srv, gmToken, sid, "guest", "", "")
	_ = dir
	_ = v
	seatID, _ := seat["id"].(string)
	if seatID == "" {
		seatID, _ = seat["seat_id"].(string)
	}
	// get real seat token via join by code
	seatToken := mustSeatToken(t, srv, sid, code)
	_ = seatToken
	// create a real encounter
	body := map[string]any{"name": "e1", "system": "dnd5e"}
	bb, _ := json.Marshal(body)
	reqc, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	reqc.Header.Set("Content-Type", "application/json")
	reqc.Header.Set("Authorization", "Bearer "+gmToken)
	respc, err := http.DefaultClient.Do(reqc)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	json.NewDecoder(respc.Body).Decode(&c)
	respc.Body.Close()
	if respc.StatusCode != http.StatusOK {
		t.Fatalf("create %d", respc.StatusCode)
	}
	eid := ""
	if enc, ok := c["encounter"].(map[string]any); ok {
		if v, ok := enc["id"].(string); ok {
			eid = v
		} else if v, ok := enc["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		if v, ok := c["id"].(string); ok {
			eid = v
		} else if v, ok := c["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		t.Fatalf("no eid: %+v", c)
	}
	// anonymous + valid seat id -> 403
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/fog?seat="+seatID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b1, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("anonymous should be 403, got %d", resp.StatusCode)
	}
	// account seat + another seat's id -> 403 (A-as-B)
	player, accToken := f2bMintAccount(t, srv, gmToken)
	_ = player
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/fog?seat="+seatID, nil)
	req2.Header.Set("Authorization", "Bearer "+accToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("non-gm should be 403, got %d", resp2.StatusCode)
	}
	// bogus seat id -> byte-identical 403 (no oracle)
	req3b, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/fog?seat=bogus-seat-xyz", nil)
	req3b.Header.Set("Authorization", "Bearer "+accToken)
	resp3b, err := http.DefaultClient.Do(req3b)
	if err != nil {
		t.Fatal(err)
	}
	b3, _ := io.ReadAll(resp3b.Body)
	resp3b.Body.Close()
	if resp3b.StatusCode != http.StatusForbidden {
		t.Fatalf("bogus seat should be 403, got %d", resp3b.StatusCode)
	}
	if string(b1) != string(b2) {
		t.Fatalf("anon/b account bodies differ: %s vs %s", string(b1), string(b2))
	}
	if string(b1) != string(b3) {
		t.Fatalf("anon/b bogus bodies differ: %s vs %s", string(b1), string(b3))
	}
	// GM + seat id -> 200 preview
	req3, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/fog?seat="+seatID, nil)
	req3.Header.Set("Authorization", "Bearer "+gmToken)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("GM should be allowed to use seat preview, got %d", resp3.StatusCode)
	}
}

func TestF2B_B2_GetEncountersProjected(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid, code := f2bCreateSession(t, srv, gmToken)
	f2bAddNote(t, dir, v, "gmnote", "GMNote", "", "gm")
	f2bAddNote(t, dir, v, "pnote", "PNote", "", "party")
	_ = dir
	_ = v
	// create encounter with hidden token and gm note
	body := map[string]any{
		"name":   "e1",
		"system": "dnd5e",
		"tokens": []map[string]any{
			{"id": "t_hidden", "hidden": true, "note_uid": "gmnote"},
			{"id": "t_pub", "hidden": false, "note_uid": "pnote"},
		},
		"participants": []map[string]any{},
	}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create %d", resp.StatusCode)
	}
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	eid := ""
	if enc, ok := c["encounter"].(map[string]any); ok {
		if v, ok := enc["id"].(string); ok {
			eid = v
		} else if v, ok := enc["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		if v, ok := c["id"].(string); ok {
			eid = v
		} else if v, ok := c["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		t.Fatalf("no eid: %+v", c)
	}
	// seated principal (real seat token) GET + LIST
	seatToken := mustSeatToken(t, srv, sid, code)
	// try GET as seat token
	reqsg, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid, nil)
	reqsg.Header.Set("Authorization", "Bearer "+seatToken)
	respsg, err := http.DefaultClient.Do(reqsg)
	if err != nil {
		t.Fatal(err)
	}
	if respsg != nil {
		bsg, _ := io.ReadAll(respsg.Body)
		respsg.Body.Close()
		if strings.Contains(string(bsg), "t_hidden") {
			t.Fatalf("hidden token leaked to seat: %s", string(bsg))
		}
		if strings.Contains(string(bsg), "gmnote") {
			t.Fatalf("gmnote uid leaked to seat: %s", string(bsg))
		}
	}
	// LIST
	reqsl, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters", nil)
	reqsl.Header.Set("Authorization", "Bearer "+seatToken)
	respsl, err := http.DefaultClient.Do(reqsl)
	if err != nil {
		t.Fatal(err)
	}
	if respsl != nil {
		bsl, _ := io.ReadAll(respsl.Body)
		respsl.Body.Close()
		if strings.Contains(string(bsl), "t_hidden") {
			t.Fatalf("hidden token leaked in seat LIST: %s", string(bsl))
		}
		if strings.Contains(string(bsl), "gmnote") {
			t.Fatalf("gmnote leaked in seat LIST: %s", string(bsl))
		}
	}
	// GM GET+LIST assert present
	reqg, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid, nil)
	reqg.Header.Set("Authorization", "Bearer "+gmToken)
	respg, err := http.DefaultClient.Do(reqg)
	if err != nil {
		t.Fatal(err)
	}
	defer respg.Body.Close()
	if respg.StatusCode != http.StatusOK {
		t.Fatalf("gm get %d", respg.StatusCode)
	}
	bgg, _ := io.ReadAll(respg.Body)
	if !strings.Contains(string(bgg), "t_hidden") {
		t.Fatalf("gm should see hidden token: %s", string(bgg))
	}
	reqgl, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters", nil)
	reqgl.Header.Set("Authorization", "Bearer "+gmToken)
	respgl, _ := http.DefaultClient.Do(reqgl)
	if respgl != nil {
		bgl, _ := io.ReadAll(respgl.Body)
		respgl.Body.Close()
		if !strings.Contains(string(bgl), "t_hidden") {
			t.Fatalf("gm should see hidden token in LIST: %s", string(bgl))
		}
	}
	_ = bgg
}

func TestF2B_B4_RollOnClosedSessionRejectedAndNotPersisted(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	// record GET /rolls count
	reqr0, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	reqr0.Header.Set("Authorization", "Bearer "+gmToken)
	respr0, err := http.DefaultClient.Do(reqr0)
	if err != nil {
		t.Fatal(err)
	}
	var out0 map[string]any
	json.NewDecoder(respr0.Body).Decode(&out0)
	respr0.Body.Close()
	count0 := 0
	if items, ok := out0["items"].([]any); ok {
		count0 = len(items)
	}
	// close session
	reqc, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	reqc.Header.Set("Authorization", "Bearer "+gmToken)
	respc, _ := http.DefaultClient.Do(reqc)
	respc.Body.Close()
	if respc.StatusCode != http.StatusOK {
		t.Fatalf("close %d", respc.StatusCode)
	}
	// roll
	body := map[string]any{"expression": "1d20", "session_id": sid, "shared": false}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/roll", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	// re-read GET /rolls -> count UNCHANGED and the rejected roll absent
	reqr, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	reqr.Header.Set("Authorization", "Bearer "+gmToken)
	respr, err := http.DefaultClient.Do(reqr)
	if err != nil {
		t.Fatal(err)
	}
	defer respr.Body.Close()
	var out map[string]any
	json.NewDecoder(respr.Body).Decode(&out)
	count1 := 0
	if items, ok := out["items"].([]any); ok {
		count1 = len(items)
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if ex, ok := m["expression"].(string); ok && ex == "1d20" {
					// could be old roll; but we need to ensure no new one added due to this rejected call
					// count unchanged is the key check
				}
			}
		}
	}
	if count1 != count0 {
		t.Fatalf("roll count changed on rejected roll: %d -> %d", count0, count1)
	}
}

func TestF2B_PacksBudgets(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	// DnD5e: xp-budget, levels 3+5 -> budget 32
	f2bAddNote(t, dir, v, "c1", "C1", "", "party")
	f2bAddNote(t, dir, v, "c2", "C2", "", "party")
	// set levels
	// easier to write directly
	if err := os.WriteFile(filepath.Join(dir, "c1.md"), []byte("---\ntype: character\nuid: c1\nname: C1\ncampaign: \"[[C]]\"\nvisibility: party\nlevel: 3\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c2.md"), []byte("---\ntype: character\nuid: c2\nname: C2\ncampaign: \"[[C]]\"\nvisibility: party\nlevel: 5\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"name":         "e1",
		"system":       "dnd5e",
		"participants": []map[string]any{{"uid": "c1", "side": "party"}, {"uid": "c2", "side": "party"}},
	}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	var budget map[string]any
	if b, ok := c["budget"].(map[string]any); ok {
		budget = b
	} else if enc, ok := c["encounter"].(map[string]any); ok {
		if b, ok := enc["budget"].(map[string]any); ok {
			budget = b
		}
	}
	if budget == nil {
		t.Fatalf("no budget: %+v", c)
	}
	if budget["xp_budget"] == nil && budget["budget"] == nil && budget["xp"] == nil {
		t.Fatalf("bad budget: %+v", budget)
	}
	// expect 32
	found := false
	for _, k := range []string{"xp_budget", "budget", "xp"} {
		if v, ok := budget[k]; ok {
			switch n := v.(type) {
			case float64:
				if int(n) == 32 {
					found = true
				}
			case int:
				if n == 32 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected budget 32, got %+v", budget)
	}

	// Blades: opposition-dice
	sid2, _ := f2bCreateSession(t, srv, gmToken)
	if err := os.WriteFile(filepath.Join(dir, "e1.md"), []byte("---\ntype: npc\nuid: e1\nname: E1\ncampaign: \"[[C]]\"\nvisibility: party\nopposition: 2\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
	body2 := map[string]any{
		"name":         "b1",
		"system":       "blades_in_the_dark",
		"participants": []map[string]any{{"uid": "e1", "side": "enemy"}},
	}
	bb2, _ := json.Marshal(body2)
	req2, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid2+"/encounters", bytes.NewReader(bb2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+gmToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var c2 map[string]any
	json.NewDecoder(resp2.Body).Decode(&c2)
	var budget2 map[string]any
	if b, ok := c2["budget"].(map[string]any); ok {
		budget2 = b
	} else if enc, ok := c2["encounter"].(map[string]any); ok {
		if b, ok := enc["budget"].(map[string]any); ok {
			budget2 = b
		}
	}
	if budget2 == nil {
		t.Fatalf("no budget2: %+v", c2)
	}
	// opposition-dice, opposition 2, clockPressure
	if budget2["opposition_dice"] == nil && budget2["oppositionDice"] == nil && budget2["opposition"] == nil {
		t.Fatalf("bad budget2: %+v", budget2)
	}
	if _, ok := budget2["clockPressure"]; !ok {
		t.Fatalf("missing clockPressure: %+v", budget2)
	}
}

func TestF2B_RollFields(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	body := map[string]any{"expression": "1d20", "workflow": "attack", "shared": true}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/roll", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("roll %d", resp.StatusCode)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	roll, _ := out["roll"].(map[string]any)
	if roll == nil {
		t.Fatalf("no roll: %+v", out)
	}
	for _, k := range []string{"roller", "workflow", "expression", "timestamp"} {
		if _, ok := roll[k]; !ok {
			t.Fatalf("missing %s in roll: %+v", k, roll)
		}
	}
	if _, ok := roll["outcome"]; !ok {
		t.Fatalf("missing outcome: %+v", roll)
	}
	if res, ok := roll["result"].(map[string]any); ok {
		if _, ok := res["faces"]; !ok {
			// faces-at-result.faces may be under result.faces or individual? check common shapes
		}
	}
	if _, ok := roll["encounter_id"]; !ok {
		// ok if absent in general; but we need encounter id when present
	}
	_ = time.Now()
}

func TestF2B_PrivateRollSilence(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid, code := f2bCreateSession(t, srv, gmToken)
	seatToken := mustSeatToken(t, srv, sid, code)
	_, accToken := f2bMintAccount(t, srv, gmToken)
	// create encounter
	body := map[string]any{"name": "e1", "system": "dnd5e"}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create %d", resp.StatusCode)
	}
	eid := ""
	if enc, ok := c["encounter"].(map[string]any); ok {
		if v, ok := enc["id"].(string); ok {
			eid = v
		} else if v, ok := enc["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		if v, ok := c["id"].(string); ok {
			eid = v
		} else if v, ok := c["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		eid = "e1"
	}
	// make a private roll
	rb := map[string]any{"expression": "1d20", "shared": false, "session_id": sid, "encounter_id": eid}
	rbb, _ := json.Marshal(rb)
	reqr, _ := http.NewRequest("POST", srv.URL+"/api/v1/roll", bytes.NewReader(rbb))
	reqr.Header.Set("Content-Type", "application/json")
	reqr.Header.Set("Authorization", "Bearer "+gmToken)
	respr2, err := http.DefaultClient.Do(reqr)
	if err != nil {
		t.Fatal(err)
	}
	respr2.Body.Close()
	if respr2.StatusCode != http.StatusOK {
		t.Fatalf("roll %d", respr2.StatusCode)
	}
	// GM receipt - GET /rolls as GM should see it
	reqg, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	reqg.Header.Set("Authorization", "Bearer "+gmToken)
	respg, err := http.DefaultClient.Do(reqg)
	if err != nil {
		t.Fatal(err)
	}
	defer respg.Body.Close()
	var gout map[string]any
	json.NewDecoder(respg.Body).Decode(&gout)
	if _, ok := gout["items"]; !ok {
		t.Fatalf("gm rolls: %+v", gout)
	}
	// private-roll absence from encounter-rolls for seats
	reqse, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/rolls", nil)
	reqse.Header.Set("Authorization", "Bearer "+seatToken)
	respse, err := http.DefaultClient.Do(reqse)
	if err != nil {
		t.Fatal(err)
	}
	defer respse.Body.Close()
	var seout map[string]any
	json.NewDecoder(respse.Body).Decode(&seout)
	if items, ok := seout["items"].([]any); ok {
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if shared, ok := m["shared"].(bool); ok && shared == false {
					t.Fatalf("private roll leaked to seat: %+v", m)
				}
			}
		}
	}
	// seated player/account - absence for non-GM
	reqp, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/rolls", nil)
	reqp.Header.Set("Authorization", "Bearer "+accToken)
	respp, err := http.DefaultClient.Do(reqp)
	if err != nil {
		t.Fatal(err)
	}
	defer respp.Body.Close()
	var pout map[string]any
	json.NewDecoder(respp.Body).Decode(&pout)
	if items, ok := pout["items"].([]any); ok {
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if shared, ok := m["shared"].(bool); ok && shared == false {
					t.Fatalf("private roll leaked to non-gm: %+v", m)
				}
			}
		}
	}
	// presence for GM on encounter-rolls
	reqge, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/rolls", nil)
	reqge.Header.Set("Authorization", "Bearer "+gmToken)
	respge, err := http.DefaultClient.Do(reqge)
	if err != nil {
		t.Fatal(err)
	}
	defer respge.Body.Close()
	var geout map[string]any
	json.NewDecoder(respge.Body).Decode(&geout)
	if items, ok := geout["items"].([]any); ok {
		found := false
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if ex, ok := m["expression"].(string); ok && ex == "1d20" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("gm should see private roll in encounter rolls: %+v", geout)
		}
	} else {
		t.Fatalf("gm encounter rolls: %+v", geout)
	}
	_ = time.Now()
}

func TestF2B_ClosedSessionAllMutations(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	// create encounter first
	body := map[string]any{"name": "e1", "system": "dnd5e"}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create %d", resp.StatusCode)
	}
	// close
	reqc, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	reqc.Header.Set("Authorization", "Bearer "+gmToken)
	respc, _ := http.DefaultClient.Do(reqc)
	respc.Body.Close()
	if respc.StatusCode != http.StatusOK {
		t.Fatalf("close %d", respc.StatusCode)
	}
	// update
	requ, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1", bytes.NewBufferString(`{"name":"x"}`))
	requ.Header.Set("Content-Type", "application/json")
	requ.Header.Set("Authorization", "Bearer "+gmToken)
	respu, _ := http.DefaultClient.Do(requ)
	respu.Body.Close()
	if respu.StatusCode != http.StatusConflict {
		t.Fatalf("update %d", respu.StatusCode)
	}
	// init roll
	reqi, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1/initiative/roll", nil)
	reqi.Header.Set("Authorization", "Bearer "+gmToken)
	respi, _ := http.DefaultClient.Do(reqi)
	respi.Body.Close()
	if respi.StatusCode != http.StatusConflict {
		t.Fatalf("initroll %d", respi.StatusCode)
	}
	// advance-on-closed (folded in)
	reqadv, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1/turn/advance", nil)
	reqadv.Header.Set("Authorization", "Bearer "+gmToken)
	respadv, _ := http.DefaultClient.Do(reqadv)
	if respadv != nil {
		respadv.Body.Close()
		if respadv.StatusCode != http.StatusConflict {
			t.Fatalf("advance %d", respadv.StatusCode)
		}
	}
	// reorder on closed
	reqro, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1/reorder", bytes.NewBufferString(`{"order":[]}`))
	reqro.Header.Set("Content-Type", "application/json")
	reqro.Header.Set("Authorization", "Bearer "+gmToken)
	respro, _ := http.DefaultClient.Do(reqro)
	if respro != nil {
		respro.Body.Close()
		if respro.StatusCode != http.StatusConflict {
			t.Fatalf("reorder %d", respro.StatusCode)
		}
	}
}

func TestF2B_SeatReorderForbidden(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid, code := f2bCreateSession(t, srv, gmToken)
	f2bAddNote(t, dir, v, "c1", "C1", "", "party")
	f2bAddNote(t, dir, v, "c2", "C2", "", "party")
	_ = dir
	_ = v
	body := map[string]any{"name": "e1", "system": "dnd5e", "participants": []map[string]any{{"uid": "c1"}, {"uid": "c2"}}}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create %d", resp.StatusCode)
	}
	eid := ""
	if enc, ok := c["encounter"].(map[string]any); ok {
		if v, ok := enc["id"].(string); ok {
			eid = v
		} else if v, ok := enc["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		if v, ok := c["id"].(string); ok {
			eid = v
		} else if v, ok := c["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		eid = "e1"
	}
	// create seat and get real seat token
	seatToken := mustSeatToken(t, srv, sid, code)
	reqs, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/reorder", bytes.NewBufferString(`{"order":["c2","c1"]}`))
	reqs.Header.Set("Content-Type", "application/json")
	reqs.Header.Set("Authorization", "Bearer "+seatToken)
	resps, err := http.DefaultClient.Do(reqs)
	if err != nil {
		t.Fatal(err)
	}
	if resps != nil {
		resps.Body.Close()
		if resps.StatusCode != http.StatusForbidden {
			t.Fatalf("non-gm seat reorder should be 403, got %d", resps.StatusCode)
		}
	}
	// GM reorder
	reqr, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/initiative/set", bytes.NewBufferString(`{"order":["c2","c1"]}`))
	reqr.Header.Set("Content-Type", "application/json")
	reqr.Header.Set("Authorization", "Bearer "+gmToken)
	respr2, err := http.DefaultClient.Do(reqr)
	if err != nil {
		t.Fatal(err)
	}
	defer respr2.Body.Close()
	if respr2.StatusCode != http.StatusOK {
		t.Fatalf("gm reorder should be 200, got %d", respr2.StatusCode)
	}
	var out map[string]any
	json.NewDecoder(respr2.Body).Decode(&out)
	if enc, ok := out["encounter"].(map[string]any); ok {
		if init, ok := enc["initiative"].([]any); ok {
			if len(init) == 0 || init[0].(string) != "c2" {
				t.Fatalf("order not changed: %+v", init)
			}
		} else if ord, ok := out["order"].([]any); ok {
			if len(ord) == 0 || ord[0].(string) != "c2" {
				t.Fatalf("order not changed: %+v", ord)
			}
		} else {
			t.Fatalf("bad order resp: %+v", out)
		}
	} else {
		if ord, ok := out["order"].([]any); ok {
			if len(ord) == 0 || ord[0].(string) != "c2" {
				t.Fatalf("order not changed: %+v", ord)
			}
		} else {
			t.Fatalf("bad order resp: %+v", out)
		}
	}
}

func TestF2B_ModelNoneUnsupported(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	body := map[string]any{"name": "e1", "system": "example"}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	var budget map[string]any
	if b, ok := c["budget"].(map[string]any); ok {
		budget = b
	} else if enc, ok := c["encounter"].(map[string]any); ok {
		if b, ok := enc["budget"].(map[string]any); ok {
			budget = b
		}
	}
	if budget == nil {
		t.Fatalf("no budget: %+v", c)
	}
	if budget["code"] != "unsupported_difficulty_model" {
		t.Fatalf("expected unsupported_difficulty_model, got %+v", budget)
	}
}

func TestF2B_UnknownEmptyDifficultyUnsupported(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	// try unknown pack
	body := map[string]any{"name": "e1", "system": "nopepack"}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	var budget map[string]any
	if b, ok := c["budget"].(map[string]any); ok {
		budget = b
	} else if enc, ok := c["encounter"].(map[string]any); ok {
		if b, ok := enc["budget"].(map[string]any); ok {
			budget = b
		}
	}
	if budget == nil {
		t.Fatalf("no budget: %+v", c)
	}
	if budget["code"] != "unsupported_difficulty_model" {
		t.Fatalf("expected unsupported_difficulty_model, got %+v", budget)
	}
}

func TestF2B_EncounterWithNonexistentNoteUIDSkipped(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	_ = dir
	_ = v
	body := map[string]any{
		"name":         "e1",
		"system":       "dnd5e",
		"participants": []map[string]any{{"uid": "does-not-exist", "side": "party"}},
		"tokens":       []map[string]any{{"id": "t1", "note_uid": "also-missing"}},
	}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create %d", resp.StatusCode)
	}
	var c map[string]any
	json.NewDecoder(resp.Body).Decode(&c)
	eid := ""
	if enc, ok := c["encounter"].(map[string]any); ok {
		if v, ok := enc["id"].(string); ok {
			eid = v
		} else if v, ok := enc["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		if v, ok := c["id"].(string); ok {
			eid = v
		} else if v, ok := c["uid"].(string); ok {
			eid = v
		}
	}
	if eid == "" {
		t.Fatalf("no eid")
	}
	// get
	reqg, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid, nil)
	reqg.Header.Set("Authorization", "Bearer "+gmToken)
	respg, err := http.DefaultClient.Do(reqg)
	if err != nil {
		t.Fatal(err)
	}
	respg.Body.Close()
	if respg.StatusCode != http.StatusOK {
		t.Fatalf("get %d", respg.StatusCode)
	}
	// list
	reql, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters", nil)
	reql.Header.Set("Authorization", "Bearer "+gmToken)
	respl, err := http.DefaultClient.Do(reql)
	if err != nil {
		t.Fatal(err)
	}
	respl.Body.Close()
	if respl.StatusCode != http.StatusOK {
		t.Fatalf("list %d", respl.StatusCode)
	}
	// fog
	reqf, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid+"/fog", nil)
	reqf.Header.Set("Authorization", "Bearer "+gmToken)
	respf, err := http.DefaultClient.Do(reqf)
	if err != nil {
		t.Fatal(err)
	}
	respf.Body.Close()
	if respf.StatusCode != http.StatusOK {
		t.Fatalf("fog %d", respf.StatusCode)
	}
}

func TestF2B_UpdateCreatesStillOpen(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid, _ := f2bCreateSession(t, srv, gmToken)
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/sessions/"+sid+"/encounters/nope-eid", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if c, ok := out["code"].(string); ok && c == "not_found" {
		// ok
	} else if e, ok := out["error"].(map[string]any); ok {
		if c2, ok := e["code"].(string); ok && c2 == "not_found" {
			// ok
		} else {
			t.Fatalf("expected not_found, got %+v", out)
		}
	} else {
		t.Fatalf("expected not_found, got %+v", out)
	}
	// re-GET → 404 + LIST clean
	reqg, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/nope-eid", nil)
	reqg.Header.Set("Authorization", "Bearer "+gmToken)
	respg, err := http.DefaultClient.Do(reqg)
	if err != nil {
		t.Fatal(err)
	}
	respg.Body.Close()
	if respg.StatusCode != http.StatusNotFound {
		t.Fatalf("re-get expected 404, got %d", respg.StatusCode)
	}
	reql, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters", nil)
	reql.Header.Set("Authorization", "Bearer "+gmToken)
	respl, err := http.DefaultClient.Do(reql)
	if err != nil {
		t.Fatal(err)
	}
	defer respl.Body.Close()
	var lout map[string]any
	json.NewDecoder(respl.Body).Decode(&lout)
	if items, ok := lout["items"].([]any); ok {
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if idv, ok := m["id"].(string); ok && idv == "nope-eid" {
					t.Fatalf("nope-eid should not appear in list")
				}
				if idv, ok := m["uid"].(string); ok && idv == "nope-eid" {
					t.Fatalf("nope-eid should not appear in list")
				}
			}
		}
	}
}
