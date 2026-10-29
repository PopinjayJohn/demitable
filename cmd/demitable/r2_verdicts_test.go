package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func verdictTestServer(t *testing.T) (string, string, string, string) {
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

func TestR2VerdictsGMFull(t *testing.T) {
	url, gmTok, _, _ := verdictTestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/partynote/verdicts", gmTok)
	if st != 200 {
		t.Fatalf("status %d: %s", st, body)
	}
	var j map[string]any
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	items, _ := j["items"].([]any)
	if len(items) != 11 {
		t.Fatalf("want 11 verdicts, got %d: %s", len(items), body)
	}
	// ref check
	refs := map[string]bool{}
	for _, it := range items {
		m := it.(map[string]any)
		refs[String(m["ref"])] = true
	}
	for _, r := range []string{"C1", "C2", "C3", "C4", "C5", "C6", "C7", "C8", "W1", "W2", "W3"} {
		if !refs[r] {
			t.Fatalf("missing ref %s", r)
		}
	}
}

func TestR2VerdictsNonGMNoGMText(t *testing.T) {
	url, _, _, partyTok := verdictTestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/partynote/verdicts", partyTok)
	if st != 200 {
		t.Fatalf("status %d: %s", st, body)
	}
	if bytes.Contains(body, []byte("secret text")) {
		t.Fatalf("GM-only secret text leaked: %s", body)
	}
	// GM-only note: hidden -> 404 empty
	st2, body2 := previewGet(t, url+"/api/v1/notes/gmnote/verdicts", partyTok)
	if st2 != 404 || len(body2) != 0 {
		t.Fatalf("hidden note status %d len %d, want 404 empty", st2, len(body2))
	}
}

func TestR2VerdictsForeignPrincipal(t *testing.T) {
	url, gmTok, ownerTok, partyTok := verdictTestServer(t)
	// non-GM naming another principal -> 403
	for _, tc := range [][2]string{{partyTok, "owner1"}, {ownerTok, "party1"}} {
		st, _ := previewGet(t, url+"/api/v1/notes/partynote/verdicts?as="+tc[1], tc[0])
		if st != 403 {
			t.Fatalf("foreign as status %d, want 403", st)
		}
	}
	// GM preview-as known account works
	st, body := previewGet(t, url+"/api/v1/notes/partynote/verdicts?as=party1", gmTok)
	if st != 200 || !bytes.Contains(body, []byte("C1")) {
		t.Fatalf("gm preview status %d", st)
	}
	// unknown target -> 404 empty
	st, body = previewGet(t, url+"/api/v1/notes/partynote/verdicts?as=nobody", gmTok)
	if st != 404 || len(body) != 0 {
		t.Fatalf("unknown target status %d len %d", st, len(body))
	}
	// anonymous naming -> 403
	st, _ = previewGet(t, url+"/api/v1/notes/partynote/verdicts?as=party1", "")
	if st != 403 {
		t.Fatalf("anon status %d", st)
	}
}

func String(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

// ensure http import used
var _ = http.MethodGet
var _ = io.Discard
var _ = strings.Contains
