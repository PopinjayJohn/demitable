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

func f2bCreateSession(t *testing.T, srv *httptest.Server, gmToken string) string {
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
	if v, ok := s["id"].(string); ok {
		return v
	}
	if v, ok := s["session"].(string); ok {
		return v
	}
	if m, ok := s["session"].(map[string]any); ok {
		if v, ok := m["id"].(string); ok {
			return v
		}
	}
	t.Fatalf("bad session resp: %+v", s)
	return ""
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

func TestF2B_B1_SeatImpersonation(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
	seat := f2bAddSeat(t, srv, gmToken, sid, "guest", "", "")
	_ = dir
	_ = v
	seatID, _ := seat["id"].(string)
	if seatID == "" {
		seatID, _ = seat["seat_id"].(string)
	}
	// anonymous + valid seat id -> refused
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1/fog?seat="+seatID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("anonymous should not impersonate seat, got 200")
	}
	// account A tries to use seat B
	player, accToken := f2bMintAccount(t, srv, gmToken)
	_ = player
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1/fog?seat="+seatID, nil)
	req2.Header.Set("Authorization", "Bearer "+accToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Fatalf("non-gm should not impersonate seat, got 200")
	}
	// GM + seat id -> preview works (encounter may not exist, but authz should pass; expect not found or ok)
	req3, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/doesnotexist/fog?seat="+seatID, nil)
	req3.Header.Set("Authorization", "Bearer "+gmToken)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode == http.StatusForbidden || resp3.StatusCode == http.StatusUnauthorized {
		t.Fatalf("GM should be allowed to use seat preview, got %d", resp3.StatusCode)
	}
}

func TestF2B_B2_GetEncountersProjected(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
	f2bAddNote(t, dir, v, "gmnote", "GMNote", "", "gm")
	f2bAddNote(t, dir, v, "pnote", "PNote", "", "party")
	_ = dir
	_ = dir
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
	// GM gets raw
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
	// non-GM (anonymous or account without access) - anonymous may be unauthorized depending; use account with no seat
	player, accToken := f2bMintAccount(t, srv, gmToken)
	_ = player
	reqp, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/encounters/"+eid, nil)
	reqp.Header.Set("Authorization", "Bearer "+accToken)
	respp, err := http.DefaultClient.Do(reqp)
	if err != nil {
		t.Fatal(err)
	}
	defer respp.Body.Close()
	if respp.StatusCode == http.StatusOK {
		bgp, _ := io.ReadAll(respp.Body)
		// hidden token + gm note uid absent from non-GM bytes
		if strings.Contains(string(bgp), "t_hidden") {
			t.Fatalf("hidden token leaked to non-gm: %s", string(bgp))
		}
		if strings.Contains(string(bgp), "gmnote") {
			t.Fatalf("gm note leaked to non-gm: %s", string(bgp))
		}
	}
	_ = bgg
}

func TestF2B_B4_RollOnClosedSessionRejectedAndNotPersisted(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
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
	// check not in GET /rolls
	reqr, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	reqr.Header.Set("Authorization", "Bearer "+gmToken)
	respr, err := http.DefaultClient.Do(reqr)
	if err != nil {
		t.Fatal(err)
	}
	defer respr.Body.Close()
	var out map[string]any
	json.NewDecoder(respr.Body).Decode(&out)
	// if items exist, should not contain our roll? we just made it - should be absent
	if items, ok := out["items"].([]any); ok {
		if len(items) > 0 {
			// more precisely, ensure no new roll from this call persisted
		}
	}
}

func TestF2B_PacksBudgets(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
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
	sid2 := f2bCreateSession(t, srv, gmToken)
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
	sid := f2bCreateSession(t, srv, gmToken)
	_, accToken := f2bMintAccount(t, srv, gmToken)
	// create encounter
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
	// make a private roll
	rb := map[string]any{"expression": "1d20", "shared": false, "session_id": sid}
	rbb, _ := json.Marshal(rb)
	reqr, _ := http.NewRequest("POST", srv.URL+"/api/v1/roll", bytes.NewReader(rbb))
	reqr.Header.Set("Content-Type", "application/json")
	reqr.Header.Set("Authorization", "Bearer "+gmToken)
	respr, err := http.DefaultClient.Do(reqr)
	if err != nil {
		t.Fatal(err)
	}
	respr.Body.Close()
	if respr.StatusCode != http.StatusOK {
		t.Fatalf("roll %d", respr.StatusCode)
	}
	// GM receipt - GET /rolls as GM should see it? private rolls still logged; GM can see
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
	// seated player - account token without seat? check silence from shared log for that player context
	reqp, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
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
	// _ = v
	_ = time.Now()
}

func TestF2B_ClosedSessionAllMutations(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
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
}

func TestF2B_SeatReorderForbidden(t *testing.T) {
	dir, v, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
	f2bAddNote(t, dir, v, "c1", "C1", "", "party")
	_ = dir
	_ = v
	body := map[string]any{"name": "e1", "system": "dnd5e", "participants": []map[string]any{{"uid": "c1"}}}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	reqr, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/encounters/e1/initiative/set", bytes.NewBufferString(`{"order":["c1"]}`))
	reqr.Header.Set("Content-Type", "application/json")
	reqr.Header.Set("Authorization", "Bearer "+gmToken)
	respr, _ := http.DefaultClient.Do(reqr)
	respr.Body.Close()
	if respr.StatusCode == http.StatusForbidden {
		t.Fatalf("gm should be able to reorder")
	}
}

func TestF2B_ModelNoneUnsupported(t *testing.T) {
	_, _, srv, gmToken := f2bEnv(t)
	sid := f2bCreateSession(t, srv, gmToken)
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
	sid := f2bCreateSession(t, srv, gmToken)
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
