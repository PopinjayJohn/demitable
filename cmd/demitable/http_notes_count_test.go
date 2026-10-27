package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func copyDemoVault(t *testing.T, dst string) {
	t.Helper()
	src := filepath.Join("..", "..", "dev", "demo-campaign", "vault")
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHTTPNotesCountGMvsAnonymous(t *testing.T) {
	dir := t.TempDir()
	copyDemoVault(t, dir)

	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})

	// GM -> 21
	greq, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes", nil)
	greq.Header.Set("Authorization", "Bearer "+gmTok)
	gresp, err := http.DefaultClient.Do(greq)
	if err != nil {
		t.Fatal(err)
	}
	var gout map[string]any
	json.NewDecoder(gresp.Body).Decode(&gout)
	io.Copy(io.Discard, gresp.Body)
	gresp.Body.Close()
	gitems, _ := gout["items"].([]any)
	if len(gitems) != 21 {
		t.Fatalf("GM count=%d want 21", len(gitems))
	}

	// anonymous -> 1
	areq, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes", nil)
	aresp, err := http.DefaultClient.Do(areq)
	if err != nil {
		t.Fatal(err)
	}
	var aout map[string]any
	json.NewDecoder(aresp.Body).Decode(&aout)
	io.Copy(io.Discard, aresp.Body)
	aresp.Body.Close()
	aitems, _ := aout["items"].([]any)
	if len(aitems) != 1 {
		t.Fatalf("anonymous count=%d want 1", len(aitems))
	}
}
