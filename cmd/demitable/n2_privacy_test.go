package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestSharedFalseInvisibleToPlayer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\n---\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// private roll
	resp, _ := http.Post(srv.URL+"/api/v1/roll", "application/json", bytes.NewReader([]byte(`{"expression":"1d20","shared":false}`)))
	resp.Body.Close()

	tk := tokenKeyFor(v)
	tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "player1"})
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r2, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	defer r2.Body.Close()
	var out map[string]any
	json.NewDecoder(r2.Body).Decode(&out)
	if c, _ := out["count"].(float64); c != 0 {
		t.Fatalf("player saw %v private rolls, want 0", c)
	}

	// GM sees it
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	req2.Header.Set("Authorization", "Bearer "+gmTok)
	r3, err := http.DefaultClient.Do(req2)
	if err != nil { t.Fatal(err) }
	defer r3.Body.Close()
	var out3 map[string]any
	json.NewDecoder(r3.Body).Decode(&out3)
	if c, _ := out3["count"].(float64); c != 1 {
		t.Fatalf("gm saw %v rolls, want 1", c)
	}
	_ = fmt.Sprint
}
