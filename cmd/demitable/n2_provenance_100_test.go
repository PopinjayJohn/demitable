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

func TestHundredRollProvenance(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\n---\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	for i := 0; i < 100; i++ {
		resp, _ := http.Post(srv.URL+"/api/v1/roll", "application/json", bytes.NewReader([]byte(`{"workflow":"attack","expression":"1d20+5","actor":"u1","target":"","shared":true}`)))
		resp.Body.Close()
	}
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/rolls", nil)
	req.Header.Set("Authorization", "Bearer "+gmTok)
	r, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	defer r.Body.Close()
	var out map[string]any
	json.NewDecoder(r.Body).Decode(&out)
	items := out["items"].([]any)
	if len(items) != 100 {
		t.Fatalf("rolls = %d, want 100", len(items))
	}
	for idx, it := range items {
		rec := it.(map[string]any)
		if rec["roller"] == nil || rec["roller"] == "" {
			t.Fatalf("roll %d missing roller", idx)
		}
		if rec["workflow"] == nil || rec["workflow"] == "" {
			t.Fatalf("roll %d missing workflow", idx)
		}
		if rec["expression"] == nil || rec["expression"] == "" {
			t.Fatalf("roll %d missing expression", idx)
		}
		if rec["individual"] == nil {
			t.Fatalf("roll %d missing individual results", idx)
		}
		if rec["modifierSources"] == nil {
			// modifierSources may be encoded as modifier_sources or modifiers; check modifiers
			if rec["modifiers"] == nil && rec["modifier_sources"] == nil {
				t.Fatalf("roll %d missing modifier sources", idx)
			}
		}
		if rec["outcome"] == nil || rec["outcome"] == "" {
			t.Fatalf("roll %d missing outcome", idx)
		}
		if rec["timestamp"] == nil && rec["ts"] == nil {
			t.Fatalf("roll %d missing timestamp", idx)
		}
		// target may be nil string; presence key
		if _, ok := rec["target"]; !ok {
			t.Fatalf("roll %d missing target", idx)
		}
	}
}
