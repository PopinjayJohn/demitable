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

func TestB2HashRebase(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nhello\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	getHash := func() string {
		resp, err := http.Get(srv.URL + "/api/v1/notes/u1")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		h, _ := out["hash"].(string)
		return h
	}
	put := func(content, hash string) (int, map[string]any) {
		body := map[string]any{"content": content}
		if hash != "" {
			body["hash"] = hash
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", srv.URL+"/api/v1/notes/u1", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	h1 := getHash()
	if h1 == "" {
		t.Fatal("GET note returned empty hash")
	}
	// concurrent change
	st, out := put("---\nuid: u1\nname: Note\nvisibility: public\n---\nconcurrent\n", h1)
	if st != 200 {
		t.Fatalf("concurrent save status %d: %v", st, out)
	}
	h2, _ := out["hash"].(string)
	if h2 == h1 {
		t.Fatal("hash did not change")
	}
	// stale save 409
	st, _ = put("---\nuid: u1\nname: Note\nvisibility: public\n---\nmine\n", h1)
	if st != http.StatusConflict {
		t.Fatalf("stale save status %d want 409", st)
	}
	// rebase: GET current
	hRebased := getHash()
	if hRebased != h2 {
		t.Fatalf("rebased hash %s want %s", hRebased, h2)
	}
	// save succeeds
	st, out = put("---\nuid: u1\nname: Note\nvisibility: public\n---\nmine2\n", hRebased)
	if st != 200 {
		t.Fatalf("rebased save status %d: %v", st, out)
	}
	h3, _ := out["hash"].(string)
	// next concurrent change 409s again
	// simulate another concurrent change
	st, out = put("---\nuid: u1\nname: Note\nvisibility: public\n---\nconcurrent2\n", h3)
	if st != 200 {
		t.Fatalf("second concurrent save status %d", st)
	}
	h4, _ := out["hash"].(string)
	st, _ = put("---\nuid: u1\nname: Note\nvisibility: public\n---\nmine3\n", h3)
	if st != http.StatusConflict {
		t.Fatalf("second stale save status %d want 409", st)
	}
	_ = h4
}
