package main

// GM-gated `?as=` preview (player eye): the GM names an account and gets
// exactly what that account would get. The account principal is built from
// the stored record — the same record a login mints a token from — so
// parity with the account's own request holds by construction, and these
// tests pin it: same bytes for note, render and list; fail-closed
// elsewhere.

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func previewTestServer(t *testing.T) (string, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".demitable"), 0o700)
	os.WriteFile(filepath.Join(dir, "gmnote.md"), []byte("---\nuid: gmnote\nname: GMNote\nvisibility: gm\ncampaign: \"[[C]]\"\n---\n> [!secret] secret text\n> body\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "partynote.md"), []byte("---\nuid: partynote\nname: PartyNote\nvisibility: party\ncampaign: \"[[C]]\"\n---\n> [!secret] secret text\n> body\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "ownernote.md"), []byte("---\nuid: ownernote\nname: OwnerNote\nvisibility: owner\nplayer: owner1\ncampaign: \"[[C]]\"\n---\n> [!secret] secret text\n> body\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".demitable", "accounts.json"), []byte(`[
{"id":"owner1","player":"owner1","campaigns":["[[C]]"],"passphrase_hash":""},
{"id":"party1","player":"party1","campaigns":["[[C]]"],"passphrase_hash":""}
]`), 0o600)

	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(muxWithVault(v))
	t.Cleanup(srv.Close)

	_, tokenKey, err := ensureVaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	gmTok, _ := issueToken(tokenKey, tokenClaims{Kind: "gm"})
	ownerTok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: "owner1", Campaigns: []string{"[[C]]"}})
	partyTok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: "party1", Campaigns: []string{"[[C]]"}})
	return srv.URL, gmTok, ownerTok, partyTok
}

func previewGet(t *testing.T, url, tok string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
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

func TestPreviewAsParity(t *testing.T) {
	url, gmTok, ownerTok, partyTok := previewTestServer(t)

	// Note parity: GM preview-as yields byte-identical bodies.
	for _, uid := range []string{"gmnote", "partynote", "ownernote"} {
		for _, tc := range [][2]string{{partyTok, "party1"}, {ownerTok, "owner1"}} {
			directSt, direct := previewGet(t, url+"/api/v1/notes/"+uid, tc[0])
			prevSt, prev := previewGet(t, url+"/api/v1/notes/"+uid+"?as="+tc[1], gmTok)
			if directSt != prevSt || !bytes.Equal(direct, prev) {
				t.Fatalf("note %s as %s: direct (%d, %d bytes) != preview (%d, %d bytes)", uid, tc[1], directSt, len(direct), prevSt, len(prev))
			}
		}
	}

	// Render parity.
	directSt, direct := previewGet(t, url+"/api/v1/notes/partynote/render", partyTok)
	prevSt, prev := previewGet(t, url+"/api/v1/notes/partynote/render?as=party1", gmTok)
	if directSt != prevSt || !bytes.Equal(direct, prev) {
		t.Fatalf("render parity: direct (%d) != preview (%d)", directSt, prevSt)
	}

	// List parity: same items in the same order.
	directSt, direct = previewGet(t, url+"/api/v1/notes", partyTok)
	prevSt, prev = previewGet(t, url+"/api/v1/notes?as=party1", gmTok)
	if directSt != prevSt || !bytes.Equal(direct, prev) {
		t.Fatalf("list parity: direct (%d, %s) != preview (%d, %s)", directSt, direct, prevSt, prev)
	}
}

func TestPreviewAsFailClosed(t *testing.T) {
	url, gmTok, ownerTok, partyTok := previewTestServer(t)

	// Unknown target: 404 with no body, like a hidden note.
	if st, body := previewGet(t, url+"/api/v1/notes/partynote?as=nobody", gmTok); st != 404 || len(body) != 0 {
		t.Fatalf("unknown target status %d len %d, want 404 empty", st, len(body))
	}
	if st, _ := previewGet(t, url+"/api/v1/notes?as=nobody", gmTok); st != 404 {
		t.Fatalf("unknown target list status %d, want 404", st)
	}

	// Non-GM naming anyone else: 403.
	for _, tc := range [][2]string{{partyTok, "owner1"}, {ownerTok, "party1"}, {partyTok, "party1-bogus"}} {
		if st, _ := previewGet(t, url+"/api/v1/notes/partynote?as="+tc[1], tc[0]); st != 403 {
			t.Fatalf("foreign as: status %d, want 403", st)
		}
	}

	// Anonymous naming anyone: 403, not a 404 oracle.
	if st, _ := previewGet(t, url+"/api/v1/notes/partynote?as=party1", ""); st != 403 {
		t.Fatalf("anonymous as: status %d, want 403", st)
	}

	// Self-naming is a harmless no-op.
	if st, _ := previewGet(t, url+"/api/v1/notes/partynote?as=party1", partyTok); st != 200 {
		t.Fatalf("self as: status %d, want 200", st)
	}

	// A GM-only note previewed as a player is absent, not stubbed.
	if st, body := previewGet(t, url+"/api/v1/notes/gmnote?as=party1", gmTok); st != 404 || len(body) != 0 {
		t.Fatalf("gmnote as party1 status %d len %d, want 404 empty", st, len(body))
	}
}

func TestPlayersListGated(t *testing.T) {
	url, gmTok, _, partyTok := previewTestServer(t)

	// GM gets the account list with no credential halves.
	st, body := previewGet(t, url+"/api/v1/players", gmTok)
	if st != 200 {
		t.Fatalf("players list GM status %d", st)
	}
	if !bytes.Contains(body, []byte("party1")) || !bytes.Contains(body, []byte("owner1")) {
		t.Fatalf("players list missing accounts: %s", body)
	}
	if bytes.Contains(body, []byte("passphrase")) || bytes.Contains(body, []byte("argon2")) {
		t.Fatalf("players list leaks credential material: %s", body)
	}

	// Non-GM is refused, anonymously or as a player.
	for _, tok := range []string{"", partyTok} {
		if st, _ := previewGet(t, url+"/api/v1/players", tok); st != 403 {
			t.Fatalf("players list non-GM status %d, want 403", st)
		}
	}
}

func TestPlayersCreateCampaign(t *testing.T) {
	url, gmTok, _, _ := previewTestServer(t)

	// GM creates a player for a named campaign; the join code carries it.
	reqBody := `{"name":"newbie","campaign":"[[C]]"}`
	req, _ := http.NewRequest("POST", url+"/api/v1/players", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create player status %d: %s", resp.StatusCode, raw)
	}
	if !bytes.Contains(raw, []byte("[[C]]")) || !bytes.Contains(raw, []byte("join_code")) {
		t.Fatalf("create player response missing campaign/code: %s", raw)
	}
}
