package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestPatchStaleHash409(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nhello\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})

	getHash := func() string {
		req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
		req.Header.Set("Authorization", "Bearer "+gmTok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		h, _ := out["hash"].(string)
		return h
	}
	patch := func(fields map[string]any, hash string) int {
		body := map[string]any{"fields": fields}
		if hash != "" {
			body["hash"] = hash
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/notes/u1", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmTok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	h1 := getHash()
	if h1 == "" {
		t.Fatal("GET note returned empty hash")
	}
	// concurrent change
	if st := patch(map[string]any{"name": "concurrent"}, h1); st != 200 {
		t.Fatalf("concurrent patch status %d want 200", st)
	}
	h2 := getHash()
	if h2 == h1 {
		t.Fatal("hash did not change")
	}
	// stale patch -> 409
	if st := patch(map[string]any{"name": "mine"}, h1); st != http.StatusConflict {
		t.Fatalf("stale patch status %d want 409", st)
	}
	// rebase: GET current hash, patch succeeds
	hRebased := getHash()
	if hRebased != h2 {
		t.Fatalf("rebased hash %s want %s", hRebased, h2)
	}
	if st := patch(map[string]any{"name": "mine2"}, hRebased); st != 200 {
		t.Fatalf("rebased patch status %d want 200", st)
	}
}
