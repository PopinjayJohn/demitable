package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestB4AnonymousNoNoteScopedEvent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: gm\n---\nhello\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})

	ac := dialWSN7(t, srv, "")
	defer ac.Close()
	readFrameN7(ac) // hello

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc) // hello

	body := []byte(`{"content":"---\nuid: u1\nname: Note\nvisibility: gm\n---\nupdated\n","hash":"` + v.GetNote("u1").Hash + `"}`)
	req, _ := http.NewRequest("PUT", srv.URL+"/api/v1/notes/u1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save status %d", resp.StatusCode)
	}

	gc.SetReadDeadline(time.Now().Add(5 * time.Second))
	gotGM := false
	for i := 0; i < 5; i++ {
		payload, err := readFrameN7(gc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(payload, &env)
		if env["type"] == "note.saved" || env["type"] == "vault.changed" {
			gotGM = true
			break
		}
	}
	if !gotGM {
		t.Fatal("GM connection did not receive note-scoped event")
	}

	ac.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		payload, err := readFrameN7(ac)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(payload, &env)
		if env["type"] == "note.saved" || env["type"] == "vault.changed" {
			t.Fatalf("anonymous received note-scoped event: %s", string(payload))
		}
	}
}
