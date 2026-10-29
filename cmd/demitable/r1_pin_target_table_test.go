package main

import (
	"encoding/json"
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

func TestR1PinTargetTableDisplayOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})
	dispTok, _ := issueToken(tk, tokenClaims{Kind: "display", Campaigns: []string{"demo"}})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	msg, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"target": "table"}},
	})
	writeFrameN7(gc, msg)
	time.Sleep(200 * time.Millisecond)

	// display principal: raw bytes must contain block text
	reqD, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	reqD.Header.Set("Authorization", "Bearer "+dispTok)
	respD, _ := http.DefaultClient.Do(reqD)
	rawD, _ := io.ReadAll(respD.Body)
	respD.Body.Close()
	if !strings.Contains(string(rawD), "hidden1") {
		t.Fatalf("display principal should see block, raw=%s", string(rawD))
	}

	// party player: raw bytes must NOT contain block text
	reqP, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	reqP.Header.Set("Authorization", "Bearer "+p1Tok)
	respP, _ := http.DefaultClient.Do(reqP)
	rawP, _ := io.ReadAll(respP.Body)
	respP.Body.Close()
	if strings.Contains(string(rawP), "hidden1") {
		t.Fatalf("party player must not see block revealed with target:table, raw=%s", string(rawP))
	}
}
