package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func f0bVaultWithNote(t *testing.T, content string) (string, *vault.Vault) {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, v
}

func f0bGMToken(v *vault.Vault) string {
	tok, _ := issueToken(tokenKeyFor(v), tokenClaims{Kind: "gm"})
	return tok
}

func f0bPlayerToken(v *vault.Vault, id string) string {
	tok, _ := issueToken(tokenKeyFor(v), tokenClaims{Kind: "account", ID: id, Campaigns: []string{"C"}})
	return tok
}

func TestF0bComputeStorm(t *testing.T) {
	dir, srv, _, gmToken := f0aSetup(t)
	_ = dir
	code := f0aMintJoinCode(t, srv, gmToken)
	resp, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	resp.Body.Close()
	playerToken := j["token"].(string)

	ok, rl := 0, 0
	for i := 0; i < 30; i++ {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/compute", bytes.NewBufferString(`{"formulas":["max_hp"],"context":{"attributes":{"rating":3}}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		r, _ := http.DefaultClient.Do(req)
		r.Body.Close()
		switch r.StatusCode {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			rl++
		}
	}
	if ok != 10 || rl != 20 {
		t.Fatalf("storm: ok=%d rl=%d, want 10/20", ok, rl)
	}
}

func TestF0bComputeUnknownFormula(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)
	resp, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	resp.Body.Close()
	playerToken := j["token"].(string)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/compute", bytes.NewBufferString(`{"formulas":["nope"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)
	r, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", r.StatusCode, string(b))
	}
	var out map[string]any
	json.Unmarshal(b, &out)
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "invalid" {
		t.Fatalf("code=%v body=%s", errObj["code"], string(b))
	}
}

func TestF0bComputeExpressionRejected(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	code := f0aMintJoinCode(t, srv, gmToken)
	resp, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	resp.Body.Close()
	playerToken := j["token"].(string)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/compute", bytes.NewBufferString(`{"expression":"os.Exit(1)"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)
	r, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", r.StatusCode, string(b))
	}
	var out map[string]any
	json.Unmarshal(b, &out)
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "invalid" {
		t.Fatalf("code=%v body=%s", errObj["code"], string(b))
	}
	if strings.Contains(string(b), "os.Exit") {
		t.Fatalf("backend text leaked: %s", string(b))
	}
}

func TestF0bDeleteInvisibleNote404(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: secret\nname: Secret\nvisibility: gm\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	playerTok := f0bPlayerToken(v, "player1")
	req, _ := http.NewRequest("DELETE", srv.URL+"/api/v1/notes/secret", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", r.StatusCode)
	}
}

func TestF0bGMDeleteJournaled(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: n1\nname: N1\nvisibility: public\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	gmTok := f0bGMToken(v)
	req, _ := http.NewRequest("DELETE", srv.URL+"/api/v1/notes/n1", nil)
	req.Header.Set("Authorization", "Bearer "+gmTok)
	r, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", r.StatusCode, string(b))
	}
	// journal entry op=delete
	jreq, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	jreq.Header.Set("Authorization", "Bearer "+gmTok)
	jr, _ := http.DefaultClient.Do(jreq)
	var jout map[string]any
	json.NewDecoder(jr.Body).Decode(&jout)
	jr.Body.Close()
	items, _ := jout["items"].([]any)
	found := false
	for _, it := range items {
		mm, _ := it.(map[string]any)
		if mm["op"] == "delete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no delete journal entry")
	}
}

func TestF0bLinksInvisibleNote404(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: secret\nname: Secret\nvisibility: gm\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	playerTok := f0bPlayerToken(v, "player1")
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/secret/links", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", r.StatusCode)
	}
}

func TestF0bJournalAccess(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: n1\nname: N1\nvisibility: public\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// anon
	r, _ := http.Get(srv.URL + "/api/v1/journal")
	r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anon status=%d", r.StatusCode)
	}
	// player
	playerTok := f0bPlayerToken(v, "player1")
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	r2, _ := http.DefaultClient.Do(req)
	r2.Body.Close()
	if r2.StatusCode != http.StatusForbidden {
		t.Fatalf("player status=%d", r2.StatusCode)
	}
}

func TestF0bRevertUnknown(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: n1\nname: N1\nvisibility: public\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	gmTok := f0bGMToken(v)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/journal/nope/revert", nil)
	req.Header.Set("Authorization", "Bearer "+gmTok)
	r, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", r.StatusCode, string(b))
	}
	var out map[string]any
	json.Unmarshal(b, &out)
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "not_found" {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestF0bPlayerRevertForbidden(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: n1\nname: N1\nvisibility: public\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	playerTok := f0bPlayerToken(v, "player1")
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/journal/je1/revert", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d", r.StatusCode)
	}
}

func TestF0bGMDeleteRevertRestores(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: n1\nname: N1\nvisibility: public\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	gmTok := f0bGMToken(v)

	// delete
	dreq, _ := http.NewRequest("DELETE", srv.URL+"/api/v1/notes/n1", nil)
	dreq.Header.Set("Authorization", "Bearer "+gmTok)
	dr, _ := http.DefaultClient.Do(dreq)
	dr.Body.Close()
	if dr.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d", dr.StatusCode)
	}

	// find journal entry id
	jreq, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	jreq.Header.Set("Authorization", "Bearer "+gmTok)
	jr, _ := http.DefaultClient.Do(jreq)
	var jout map[string]any
	json.NewDecoder(jr.Body).Decode(&jout)
	jr.Body.Close()
	items, _ := jout["items"].([]any)
	var entryID string
	for _, it := range items {
		mm, _ := it.(map[string]any)
		if mm["op"] == "delete" {
			entryID = mm["id"].(string)
		}
	}
	if entryID == "" {
		t.Fatal("no delete entry")
	}

	// revert
	rreq, _ := http.NewRequest("POST", srv.URL+"/api/v1/journal/"+entryID+"/revert", nil)
	rreq.Header.Set("Authorization", "Bearer "+gmTok)
	rr, _ := http.DefaultClient.Do(rreq)
	b, _ := io.ReadAll(rr.Body)
	rr.Body.Close()
	if rr.StatusCode != http.StatusOK {
		t.Fatalf("revert status=%d body=%s", rr.StatusCode, string(b))
	}
	// note restored
	gr, _ := http.Get(srv.URL + "/api/v1/notes/n1")
	gr.Body.Close()
	if gr.StatusCode != http.StatusOK {
		t.Fatalf("note not restored: status=%d", gr.StatusCode)
	}
}

func TestF0bUploadSizeRejected(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	data := make([]byte, 10<<20+1)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/assets", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode < 400 {
		t.Fatalf("status=%d, want rejection", r.StatusCode)
	}
}

func TestF0bUploadExeRejected(t *testing.T) {
	_, srv, _, gmToken := f0aSetup(t)
	data := []byte("MZ\x90\x00") // exe-like
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/assets", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	r, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status=%d body=%s", r.StatusCode, string(b))
	}
}

func TestF0bUploadAttackPaths(t *testing.T) {
	dir, srv, _, gmToken := f0aSetup(t)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	for _, pth := range []string{".demitable/pwned.png", "Notes/evil.md", ".demitable/accounts.json"} {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/assets?path="+pth, bytes.NewReader(png))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		r, _ := http.DefaultClient.Do(req)
		r.Body.Close()
		if r.StatusCode < 400 {
			t.Errorf("path=%s status=%d, want rejection", pth, r.StatusCode)
		}
		full := filepath.Join(dir, filepath.FromSlash(pth))
		if _, err := os.Stat(full); err == nil {
			t.Errorf("file exists on disk: %s", full)
		}
	}
}

func TestF0bAssetExcludedCaller(t *testing.T) {
	dir, srv, _, gmToken := f0aSetup(t)
	// upload an asset as GM
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/assets", bytes.NewReader(png))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	r, _ := http.DefaultClient.Do(req)
	var resp map[string]any
	json.NewDecoder(r.Body).Decode(&resp)
	r.Body.Close()
	assetPath := resp["path"].(string)

	// excluded caller (player not referencing) GET -> 404, 0 bytes
	_ = dir
	areq, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	ar, _ := http.DefaultClient.Do(areq)
	body, _ := io.ReadAll(ar.Body)
	ar.Body.Close()
	if ar.StatusCode != http.StatusNotFound {
		t.Fatalf("anon status=%d", ar.StatusCode)
	}
	if len(body) != 0 {
		t.Fatalf("body bytes=%d, want 0", len(body))
	}

	// GM -> 200 full bytes
	greq, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	greq.Header.Set("Authorization", "Bearer "+gmToken)
	gr, _ := http.DefaultClient.Do(greq)
	gbody, _ := io.ReadAll(gr.Body)
	gr.Body.Close()
	if gr.StatusCode != http.StatusOK {
		t.Fatalf("gm status=%d", gr.StatusCode)
	}
	if len(gbody) != len(png) {
		t.Fatalf("gm bytes=%d want %d", len(gbody), len(png))
	}

	// If-None-Match -> 304
	etag := gr.Header.Get("ETag")
	// recompute etag from first response? use stored
	_ = etag
	// fetch etag first
	greq2, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	greq2.Header.Set("Authorization", "Bearer "+gmToken)
	gr2, _ := http.DefaultClient.Do(greq2)
	etag = gr2.Header.Get("ETag")
	gr2.Body.Close()
	inmReq, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	inmReq.Header.Set("Authorization", "Bearer "+gmToken)
	inmReq.Header.Set("If-None-Match", etag)
	inmr, _ := http.DefaultClient.Do(inmReq)
	inmr.Body.Close()
	if inmr.StatusCode != http.StatusNotModified {
		t.Fatalf("inm status=%d", inmr.StatusCode)
	}

	// Range bytes=0-3 -> 206 with exact 4 bytes
	rr, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	rr.Header.Set("Authorization", "Bearer "+gmToken)
	rr.Header.Set("Range", "bytes=0-3")
	rrr, _ := http.DefaultClient.Do(rr)
	rbody, _ := io.ReadAll(rrr.Body)
	rrr.Body.Close()
	if rrr.StatusCode != http.StatusPartialContent {
		t.Fatalf("range status=%d", rrr.StatusCode)
	}
	if len(rbody) != 4 {
		t.Fatalf("range bytes=%d want 4", len(rbody))
	}
}

func TestF0bForgedExpiredBearer(t *testing.T) {
	dir, srv, _, gmToken := f0aSetup(t)
	_ = gmToken
	_ = dir
	// forged
	forged := "aaaa.bbbb"
	// expired token signed with key? need key; use gmToken? We'll just use malformed.
	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/compute"},
		{"GET", "/api/v1/journal"},
		{"POST", "/api/v1/assets"},
		{"POST", "/api/v1/journal/x/revert"},
	}
	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, srv.URL+ep.path, bytes.NewReader([]byte("{}")))
		req.Header.Set("Authorization", "Bearer "+forged)
		r, _ := http.DefaultClient.Do(req)
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s forged: status=%d want 401", ep.method, ep.path, r.StatusCode)
		}
	}
	// expired token signed with the vault key
	v, _ := vault.Open(dir)
	key := tokenKeyFor(v)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"kind":"account","id":"player1","exp":1}`))
	sig := signToken(key, payload)
	expired := payload + "." + sig
	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, srv.URL+ep.path, bytes.NewReader([]byte("{}")))
		req.Header.Set("Authorization", "Bearer "+expired)
		r, _ := http.DefaultClient.Do(req)
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s expired: status=%d want 401", ep.method, ep.path, r.StatusCode)
		}
	}
}

func TestF0bSetPassphraseNoJoinBearer401(t *testing.T) {
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

	bb, _ := json.Marshal(map[string]any{"player": player, "passphrase": "hunter2"})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/set-passphrase", bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
	_ = gmToken
}

func TestF0bLinksInvisibleTargetNoUIDLeak(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "visible.md"), []byte("---\nuid: visible\nname: Visible\nvisibility: public\n---\nSee `[[Secret]]`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.md"), []byte("---\nuid: secretuid\nname: Secret\nvisibility: gm\n---\nhidden body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	playerTok := f0bPlayerToken(v, "player1")
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/visible/links", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", r.StatusCode, string(b))
	}
	if strings.Contains(string(b), "secretuid") {
		t.Fatalf("hidden uid leaked in links response: %s", string(b))
	}
}

func TestF0bSecretBlockAssetStays404(t *testing.T) {
	dir := t.TempDir()
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	gmTok := f0bGMToken(v)

	// upload an asset as GM
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/assets", bytes.NewReader(png))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+gmTok)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	json.NewDecoder(r.Body).Decode(&resp)
	r.Body.Close()
	assetPath := resp["path"].(string)

	// visible note references the asset ONLY inside a secret block
	content := "---\nuid: n1\nname: N1\nvisibility: public\n---\nVisible note body.\n\n> [!secret] hidden\n> " + assetPath + "\n"
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}

	playerTok := f0bPlayerToken(v, "player1")

	// player fetch -> 404, 0 bytes
	areq, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	areq.Header.Set("Authorization", "Bearer "+playerTok)
	ar, err := http.DefaultClient.Do(areq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(ar.Body)
	ar.Body.Close()
	if ar.StatusCode != http.StatusNotFound {
		t.Fatalf("player status=%d want 404", ar.StatusCode)
	}
	if len(body) != 0 {
		t.Fatalf("player body bytes=%d want 0", len(body))
	}

	// GM fetch -> 200
	greq, _ := http.NewRequest("GET", srv.URL+"/asset/"+assetPath, nil)
	greq.Header.Set("Authorization", "Bearer "+gmTok)
	gr, err := http.DefaultClient.Do(greq)
	if err != nil {
		t.Fatal(err)
	}
	gbody, _ := io.ReadAll(gr.Body)
	gr.Body.Close()
	if gr.StatusCode != http.StatusOK {
		t.Fatalf("gm status=%d want 200", gr.StatusCode)
	}
	if len(gbody) != len(png) {
		t.Fatalf("gm bytes=%d want %d", len(gbody), len(png))
	}
}

func TestF0bApproveRejectJournalEntry(t *testing.T) {
	dir, v := f0bVaultWithNote(t, "---\nuid: c1\nname: C1\nvisibility: public\nworkflow: submitted\n---\nbody\n")
	_ = dir
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	gmTok := f0bGMToken(v)

	// approve
	areq, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters/c1/approve", nil)
	areq.Header.Set("Authorization", "Bearer "+gmTok)
	ar, err := http.DefaultClient.Do(areq)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(ar.Body)
	ar.Body.Close()
	if ar.StatusCode != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", ar.StatusCode, string(b))
	}
	// workflow becomes exactly active after approve.
	var aout map[string]any
	_ = json.Unmarshal(b, &aout)
	if aout["workflow"] != "active" {
		t.Fatalf("approve response workflow=%v want active", aout["workflow"])
	}
	if n := v.GetNote("c1"); n == nil || n.Workflow != "active" {
		t.Fatalf("workflow after approve=%v want active", func() any {
			if n == nil {
				return nil
			}
			return n.Workflow
		}())
	}

	// journal lists the entry
	jreq, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	jreq.Header.Set("Authorization", "Bearer "+gmTok)
	jr, err := http.DefaultClient.Do(jreq)
	if err != nil {
		t.Fatal(err)
	}
	var jout map[string]any
	json.NewDecoder(jr.Body).Decode(&jout)
	jr.Body.Close()
	items, _ := jout["items"].([]any)
	var entryID string
	for _, it := range items {
		mm, _ := it.(map[string]any)
		if mm["uid"] == "c1" {
			entryID = mm["id"].(string)
		}
	}
	if entryID == "" {
		t.Fatal("no journal entry for c1")
	}

	// revert
	rreq, _ := http.NewRequest("POST", srv.URL+"/api/v1/journal/"+entryID+"/revert", nil)
	rreq.Header.Set("Authorization", "Bearer "+gmTok)
	rr, err := http.DefaultClient.Do(rreq)
	if err != nil {
		t.Fatal(err)
	}
	rr.Body.Close()
	if rr.StatusCode != http.StatusOK {
		t.Fatalf("revert status=%d", rr.StatusCode)
	}

	// workflow restored to submitted
	if n := v.GetNote("c1"); n == nil || n.Workflow != "submitted" {
		t.Fatalf("workflow=%v want submitted", func() any {
			if n == nil {
				return nil
			}
			return n.Workflow
		}())
	}
}
