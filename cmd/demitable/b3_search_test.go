package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestB3SearchSnippet(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Salt Note\nvisibility: public\n---\nthe quick brown salt fox\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})

	q := `name contains "salt" or body contains "salt"`
	u := srv.URL + "/api/v1/notes?q=" + url.QueryEscape(q)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+gmTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Items) == 0 {
		t.Fatal("no results")
	}
	sn, _ := out.Items[0]["snippet"].(string)
	if sn == "" {
		t.Fatalf("missing snippet")
	}
	if !strings.Contains(strings.ToLower(sn), "salt") {
		t.Fatalf("snippet not from matching passage: %s", sn)
	}
}
