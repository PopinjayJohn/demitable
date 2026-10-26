package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

func n6Setup(t *testing.T) (string, *httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	if err := runInit([]string{dir}); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	// GM auth
	gmSecret, _, err := ensureVaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = gmSecret
	resp, err := http.Post(srv.URL+"/api/v1/auth/gm", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"secret":%q}`, gmSecret)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	token := body["token"].(string)
	return dir, srv, token
}

func n6GMToken(t *testing.T, srv *httptest.Server, secret string) string {
	resp, err := http.Post(srv.URL+"/api/v1/auth/gm", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"secret":%q}`, secret)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	return body["token"].(string)
}

func TestN6WizardCreatesPerPack(t *testing.T) {
	dir, srv, gmToken := n6Setup(t)
	_ = dir
	for _, sys := range []string{"dnd5e", "blades_in_the_dark"} {
		payload := map[string]any{
			"system":     sys,
			"name":       "WizardChar-" + sys,
			"attributes": map[string]any{"str": 10},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("%s: status %d", sys, resp.StatusCode)
		}
		var created map[string]any
		json.NewDecoder(resp.Body).Decode(&created)
		resp.Body.Close()
		uid := created["uid"].(string)
		// sheet
		req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/characters/"+uid+"?expand=1", nil)
		req2.Header.Set("Authorization", "Bearer "+gmToken)
		resp2, _ := http.DefaultClient.Do(req2)
		var sheet map[string]any
		json.NewDecoder(resp2.Body).Decode(&sheet)
		resp2.Body.Close()
		pk, _ := pack.Load(sys)
		var ct *pack.ContentType
		for i := range pk.Content {
			if pk.Content[i].ID == "character" {
				ct = &pk.Content[i]
			}
		}
		if ct == nil {
			t.Fatalf("%s: no character content type", sys)
		}
		declared := sheet["declared"].([]any)
		if len(declared) != len(ct.Fields) {
			t.Fatalf("%s: declared %d != pack %d", sys, len(declared), len(ct.Fields))
		}
	}
}

func TestN6NonGMCannotSelfApprove(t *testing.T) {
	dir, srv, gmToken := n6Setup(t)
	_ = dir
	// mint join code + account
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"player1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	var pc map[string]any
	json.NewDecoder(resp.Body).Decode(&pc)
	resp.Body.Close()
	code := pc["join_code"].(string)
	// redeem
	r2, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	var jc map[string]any
	json.NewDecoder(r2.Body).Decode(&jc)
	r2.Body.Close()
	acctToken := jc["token"].(string)
	// try self-approve
	payload := map[string]any{"system": "example", "name": "SelfAppr", "workflow": "active"}
	b, _ := json.Marshal(payload)
	req3, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+acctToken)
	resp3, _ := http.DefaultClient.Do(req3)
	var created map[string]any
	json.NewDecoder(resp3.Body).Decode(&created)
	resp3.Body.Close()
	if created["workflow"] != "draft" {
		t.Fatalf("non-GM self-approve: workflow=%v", created["workflow"])
	}
	// GM one-step
	payload2 := map[string]any{"system": "example", "name": "GMAppr"}
	b2, _ := json.Marshal(payload2)
	req4, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b2))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer "+gmToken)
	resp4, _ := http.DefaultClient.Do(req4)
	var created2 map[string]any
	json.NewDecoder(resp4.Body).Decode(&created2)
	resp4.Body.Close()
	if created2["workflow"] != "active" {
		t.Fatalf("GM one-step: workflow=%v", created2["workflow"])
	}
}

func TestN6StateMachineSubmittedReadOnly(t *testing.T) {
	dir, srv, gmToken := n6Setup(t)
	_ = dir
	// mint a real player account
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"player1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	var pc map[string]any
	json.NewDecoder(resp.Body).Decode(&pc)
	resp.Body.Close()
	code := pc["join_code"].(string)
	r2, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	var jc map[string]any
	json.NewDecoder(r2.Body).Decode(&jc)
	r2.Body.Close()
	playerToken := jc["token"].(string)

	// player creates character (draft, owned)
	payload := map[string]any{"system": "example", "name": "SMChar"}
	b, _ := json.Marshal(payload)
	reqC, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	reqC.Header.Set("Content-Type", "application/json")
	reqC.Header.Set("Authorization", "Bearer "+playerToken)
	respC, _ := http.DefaultClient.Do(reqC)
	var created map[string]any
	json.NewDecoder(respC.Body).Decode(&created)
	respC.Body.Close()
	uid := created["uid"].(string)

	// player submits
	reqS, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters/"+uid+"/submit", nil)
	reqS.Header.Set("Authorization", "Bearer "+playerToken)
	respS, _ := http.DefaultClient.Do(reqS)
	respS.Body.Close()
	if respS.StatusCode != 200 {
		t.Fatalf("submit status %d", respS.StatusCode)
	}

	// player PUT submitted -> 403
	reqP, _ := http.NewRequest("PUT", srv.URL+"/api/v1/notes/"+uid, bytes.NewBufferString(`{"content":"---\nuid: x\n---\nhi"}`))
	reqP.Header.Set("Content-Type", "application/json")
	reqP.Header.Set("Authorization", "Bearer "+playerToken)
	respP, _ := http.DefaultClient.Do(reqP)
	respP.Body.Close()
	if respP.StatusCode != 403 {
		t.Fatalf("player PUT submitted: got %d want 403", respP.StatusCode)
	}

	// player PATCH submitted -> 403
	reqPa, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/notes/"+uid, bytes.NewBufferString(`{"fields":{"name":"X"}}`))
	reqPa.Header.Set("Content-Type", "application/json")
	reqPa.Header.Set("Authorization", "Bearer "+playerToken)
	respPa, _ := http.DefaultClient.Do(reqPa)
	respPa.Body.Close()
	if respPa.StatusCode != 403 {
		t.Fatalf("player PATCH submitted: got %d want 403", respPa.StatusCode)
	}

	// GM can edit submitted (PATCH workflow retired)
	reqG, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/notes/"+uid, bytes.NewBufferString(`{"fields":{"workflow":"retired"}}`))
	reqG.Header.Set("Content-Type", "application/json")
	reqG.Header.Set("Authorization", "Bearer "+gmToken)
	respG, _ := http.DefaultClient.Do(reqG)
	respG.Body.Close()
	if respG.StatusCode != 200 {
		t.Fatalf("GM edit submitted: got %d want 200", respG.StatusCode)
	}
}

func TestN6WizardNoteNotInvisibleToAll(t *testing.T) {
	dir, srv, gmToken := n6Setup(t)
	// mint a real player account
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"player1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	var pc map[string]any
	json.NewDecoder(resp.Body).Decode(&pc)
	resp.Body.Close()
	code := pc["join_code"].(string)
	r2, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	var jc map[string]any
	json.NewDecoder(r2.Body).Decode(&jc)
	r2.Body.Close()
	playerID := jc["player"].(string)

	// create wizard note as player
	payload := map[string]any{"system": "example", "name": "VisChar", "visibility": "private"}
	b, _ := json.Marshal(payload)
	reqC, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	reqC.Header.Set("Content-Type", "application/json")
	reqC.Header.Set("Authorization", "Bearer "+jc["token"].(string))
	respC, _ := http.DefaultClient.Do(reqC)
	respC.Body.Close()

	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	realPrincipals := []vault.Principal{vault.GM(), vault.Account(playerID, "[[Campaign]]")}
	for _, n := range v.AllNotes() {
		seen := false
		for _, pr := range realPrincipals {
			if vault.Visible(pr, *n) {
				seen = true
				break
			}
		}
		if !seen {
			t.Fatalf("wizard-created note invisible to all real principals: %s", n.Path)
		}
	}
}

func TestN6PackFieldsNonVacuousAllPacks(t *testing.T) {
	for _, id := range []string{"example", "dnd5e", "blades_in_the_dark"} {
		pk, err := pack.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		owner, gm := 0, 0
		for _, ct := range pk.Content {
			for _, f := range ct.Fields {
				switch f.Visibility {
				case "owner":
					owner++
				case "gm":
					gm++
				}
			}
		}
		if owner == 0 || gm == 0 {
			t.Fatalf("%s: owner=%d gm=%d", id, owner, gm)
		}
	}
}

func TestN6EffectiveVisibilityEndpoint(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	payload := map[string]any{"system": "example", "name": "EffChar", "visibility": "party"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	uid := created["uid"].(string)
	// get note
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/"+uid, nil)
	req2.Header.Set("Authorization", "Bearer "+gmToken)
	resp2, _ := http.DefaultClient.Do(req2)
	var note map[string]any
	json.NewDecoder(resp2.Body).Decode(&note)
	resp2.Body.Close()
	if note["effective_visibility"] != "party" {
		t.Fatalf("effective_visibility=%v", note["effective_visibility"])
	}
}

func TestN6SheetByteMatch(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	cases := []struct {
		sys string
	}{
		{"dnd5e"}, {"blades_in_the_dark"},
	}
	for _, c := range cases {
		payload := map[string]any{
			"system": c.sys,
			"name":   "ByteChar-" + c.sys,
			"fields": map[string]any{"alias": "x", "class": "fighter", "name": "ByteChar-" + c.sys},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		resp, _ := http.DefaultClient.Do(req)
		var created map[string]any
		json.NewDecoder(resp.Body).Decode(&created)
		resp.Body.Close()
		uid := created["uid"].(string)

		// frontmatter from notes endpoint
		reqN, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/"+uid, nil)
		reqN.Header.Set("Authorization", "Bearer "+gmToken)
		respN, _ := http.DefaultClient.Do(reqN)
		var note map[string]any
		json.NewDecoder(respN.Body).Decode(&note)
		respN.Body.Close()
		content := note["content"].(string)

		req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/characters/"+uid+"?expand=1", nil)
		req2.Header.Set("Authorization", "Bearer "+gmToken)
		resp2, _ := http.DefaultClient.Do(req2)
		var sheet map[string]any
		json.NewDecoder(resp2.Body).Decode(&sheet)
		resp2.Body.Close()

		// declared == pack
		pk, _ := pack.Load(c.sys)
		var ct *pack.ContentType
		for i := range pk.Content {
			if pk.Content[i].ID == "character" {
				ct = &pk.Content[i]
			}
		}
		declared := sheet["declared"].([]any)
		if len(declared) != len(ct.Fields) {
			t.Fatalf("%s: declared %d != pack %d", c.sys, len(declared), len(ct.Fields))
		}

		// parse frontmatter data map
		doc, err := frontmatter.Parse([]byte(content))
		if err != nil || !doc.HasBlock {
			t.Fatal(err)
		}
		fmData := map[string]any{}
		for _, f := range doc.Block.Fields {
			if f.Key == "data" {
				fmData = fmValueToAny(frontmatter.ValueOf(f.ValueNode)).(map[string]any)
			}
		}
		fields := sheet["fields"].(map[string]any)
		for k, v := range fmData {
			if sv, ok := fields[k]; ok {
				if fmt.Sprintf("%v", sv) != fmt.Sprintf("%v", v) {
					t.Fatalf("%s: field %s sheet=%v fm=%v", c.sys, k, sv, v)
				}
			}
		}
	}
}

func TestN6EndpointShapes(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	// packs list
	resp, _ := http.Get(srv.URL + "/api/v1/packs")
	var pl map[string]any
	json.NewDecoder(resp.Body).Decode(&pl)
	resp.Body.Close()
	if pl["count"].(float64) < 2 {
		t.Fatalf("packs count %v", pl["count"])
	}
	// packs show
	resp2, _ := http.Get(srv.URL + "/api/v1/packs/dnd5e")
	var ps map[string]any
	json.NewDecoder(resp2.Body).Decode(&ps)
	resp2.Body.Close()
	if ps["id"] != "dnd5e" {
		t.Fatalf("pack id %v", ps["id"])
	}
	// validate
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/packs/dnd5e/validate", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp3, _ := http.DefaultClient.Do(req)
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("validate status %d", resp3.StatusCode)
	}
	// content
	resp4, _ := http.Get(srv.URL + "/api/v1/packs/example/content?content_type=character")
	var pc map[string]any
	json.NewDecoder(resp4.Body).Decode(&pc)
	resp4.Body.Close()
	if _, ok := pc["items"]; !ok {
		t.Fatal("content missing items")
	}
	// submit 404 for unknown
	resp5, _ := http.Post(srv.URL+"/api/v1/characters/NOPE/submit", "application/json", nil)
	resp5.Body.Close()
	if resp5.StatusCode != 404 {
		t.Fatalf("submit unknown status %d", resp5.StatusCode)
	}
}

func TestN6BadgeAgreesWithProjection(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	// create a draft note owned by nobody? use GM creates draft private?
	payload := map[string]any{"system": "example", "name": "BadgeChar", "visibility": "party", "workflow": "draft"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	uid := created["uid"].(string)

	// anonymous requester: projection denies (party note, no membership) -> badge gm
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/"+uid, nil)
	resp2, _ := http.DefaultClient.Do(req2)
	var note map[string]any
	json.NewDecoder(resp2.Body).Decode(&note)
	resp2.Body.Close()
	if note["visible"] != false {
		t.Fatalf("expected not visible, got %v", note["visible"])
	}
	if note["effective_visibility"] != "gm" {
		t.Fatalf("badge label %v disagrees with projection", note["effective_visibility"])
	}
}
