package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestVaultTableShadowsPack(t *testing.T) {
	dir := t.TempDir()
	note := "---\nuid: t1\nname: shadow\ntype: table\ndata:\n  faces:\n    - id: vault-face\n      label: VaultFace\n      symbol: \"V\"\n---\n"
	os.WriteFile(filepath.Join(dir, "shadow.md"), []byte(note), 0o644)
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/v1/roll", "application/json", strings.NewReader(`{"expression":"1dshadow","shared":true}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	res := out["result"].(map[string]any)
	faces := res["faces"].([]any)
	if len(faces) == 0 {
		t.Fatal("no faces")
	}
	face := faces[0].(map[string]any)
	if face["face"] != nil {
		fm := face["face"].(map[string]any)
		if fm["label"] != "VaultFace" && fm["symbol"] != "V" {
			t.Fatalf("expected vault face, got %+v", fm)
		}
	}
	// also check roll result JSON contains VaultFace somewhere
	raw, _ := json.Marshal(out)
	if !strings.Contains(string(raw), "VaultFace") {
		t.Fatalf("vault face not in response: %s", string(raw))
	}
}
