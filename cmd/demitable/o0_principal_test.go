package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestO0PrincipalChecks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".demitable"), 0o700)
	os.WriteFile(filepath.Join(dir, "gmnote.md"), []byte("---\nuid: gmnote\nname: GMNote\nvisibility: gm\ncampaign: \"[[C]]\"\n---\n> [!secret] secret text\n> body ^blk1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "partynote.md"), []byte("---\nuid: partynote\nname: PartyNote\nvisibility: party\ncampaign: \"[[C]]\"\n---\n> [!secret] secret text\n> body ^blk2\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "ownernote.md"), []byte("---\nuid: ownernote\nname: OwnerNote\nvisibility: owner\nplayer: owner1\ncampaign: \"[[C]]\"\n---\n> [!secret] secret text\n> body ^blk3\n"), 0o644)

	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	_, tokenKey, err := ensureVaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	gmTok, _ := issueToken(tokenKey, tokenClaims{Kind: "gm"})
	ownerTok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: "owner1", Campaigns: []string{"[[C]]"}})
	partyTok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: "party1", Campaigns: []string{"[[C]]"}})

	get := func(tok, uid string) (int, []byte) {
		req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/"+uid, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, b
	}

	// GM-only note: GM sees raw; others 404.
	st, body := get(gmTok, "gmnote")
	if st != 200 {
		t.Fatalf("GM gmnote status %d", st)
	}
	if !containsBytes(body, []byte("secret text")) {
		t.Fatalf("GM did not get raw secret text: %s", body)
	}
	for _, tok := range []string{ownerTok, partyTok, ""} {
		st, body := get(tok, "gmnote")
		if st != 404 {
			t.Fatalf("principal gmnote status %d want 404: %s", st, body)
		}
		if len(body) != 0 {
			t.Fatalf("expected zero body bytes, got %d", len(body))
		}
	}

	// Party-visible note: non-GM gets projected body (no secret text), never raw.
	st, body = get(partyTok, "partynote")
	if st != 200 {
		t.Fatalf("party partynote status %d: %s", st, body)
	}
	if containsBytes(body, []byte("secret text")) {
		t.Fatalf("party got raw secret text: %s", body)
	}

	// Owner-visible note: owner gets projected body.
	st, body = get(ownerTok, "ownernote")
	if st != 200 {
		t.Fatalf("owner ownernote status %d: %s", st, body)
	}
	if containsBytes(body, []byte("secret text")) {
		t.Fatalf("owner got raw secret text: %s", body)
	}

	// GM sees raw on party note.
	st, body = get(gmTok, "partynote")
	if st != 200 || !containsBytes(body, []byte("secret text")) {
		t.Fatalf("GM partynote not raw: %d %s", st, body)
	}

	// Render endpoint: GM-only note -> 404 for non-GM.
	st, body = renderGet(srv.URL, partyTok, "gmnote")
	if st != 404 || len(body) != 0 {
		t.Fatalf("render gmnote party status %d len %d", st, len(body))
	}
	// Render party note -> projected (no secret text) for party.
	st, body = renderGet(srv.URL, partyTok, "partynote")
	if st != 200 || containsBytes(body, []byte("secret text")) {
		t.Fatalf("render partynote party leaked: %d %s", st, body)
	}
	_ = fmt.Sprint
}

func renderGet(srvURL, tok, uid string) (int, []byte) {
	req, _ := http.NewRequest("GET", srvURL+"/api/v1/notes/"+uid+"/render", nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp.StatusCode, b
}

func containsBytes(b, sub []byte) bool {
	return len(b) > 0 && (func() bool {
		for i := 0; i+len(sub) <= len(b); i++ {
			if string(b[i:i+len(sub)]) == string(sub) {
				return true
			}
		}
		return false
	})()
}
